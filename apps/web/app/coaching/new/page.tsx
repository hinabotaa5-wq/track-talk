"use client";

import { useAuth } from "@/components/auth-provider";
import { DemoCodeForm } from "@/components/coaching-demo";
import { ArrowRight } from "lucide-react";
import { Textarea } from "@/components/ui/textarea";
import { Button } from "@/components/ui/button";
import { UploadProgress } from "@/components/upload-progress";
import { apiFetch, apiErrorText } from "@/lib/api";
import { coachEventLabels } from "@/lib/coach-recruit";
import { uploadToCloudinary, type CloudinarySignature, type CloudinaryUploadProgress } from "@/lib/cloudinary-upload";
import {
  maxReviewSeconds,
  reviewEvents,
  reviewPayMethodLabels,
  tooLong,
  videoDuration,
} from "@/lib/review";
import type { TicketSummary } from "@/lib/coaching-tickets";
import { formatDateTime, formatYen } from "@/lib/format";
import type { CoachEvent, CoachingMe, ReviewEstimate, ReviewPayMethod, ReviewRequest } from "@/lib/types";
import { useActionLock } from "@/lib/use-action-lock";
import Link from "next/link";
import { useRouter } from "next/navigation";
import { useEffect, useState, type FormEvent } from "react";

// フォーム添削の依頼を送る。いまは招待コードを入れた人だけが使える。
// Stripe の鍵があるときは、送るとそのまま支払い画面へ進み、支払いが済んだらコーチを探し始める。
// 鍵がない間は支払いなしのデモ版。

const maxBytes = 100 * 1024 * 1024;

const chip = (on: boolean) =>
  `rounded-full border px-3 py-1.5 text-sm transition active:scale-[0.98] ${
    on ? "border-blue-400 bg-blue-500/30 text-white" : "border-white/25 bg-white/10 text-slate-200"
  }`;

export default function NewReviewRequestPage() {
  const { session, token, loading } = useAuth();
  const router = useRouter();
  const [me, setMe] = useState<CoachingMe | null>(null);
  const [loadError, setLoadError] = useState("");
  const tier = "coach";
  const [event, setEvent] = useState<CoachEvent>(reviewEvents[0]);
  const [question, setQuestion] = useState("");
  const [file, setFile] = useState<File | null>(null);
  const [error, setError] = useState("");
  const [upload, setUpload] = useState<CloudinaryUploadProgress | null>(null);
  const [uploadPhase, setUploadPhase] = useState<"upload" | "saving">("upload");
  const [estimate, setEstimate] = useState<ReviewEstimate | null>(null);
  const [ticketSummary, setTicketSummary] = useState<TicketSummary | null>(null);
  const [method, setMethod] = useState<ReviewPayMethod | "tickets">("card");
  const { pending, acquire, release } = useActionLock();

  useEffect(() => {
    if (!loading && !session) {
      router.replace(`/login?next=${encodeURIComponent(window.location.pathname)}`);
    }
  }, [loading, session, router]);

  async function loadMe() {
    try {
      setMe(await apiFetch<CoachingMe>("/v1/me/coaching", { token }));
    } catch {
      setLoadError("読み込みに失敗しました");
    }
  }

  useEffect(() => {
    if (!token) return;
    let cancelled = false;
    apiFetch<CoachingMe>("/v1/me/coaching", { token })
      .then((d) => {
        if (!cancelled) setMe(d);
      })
      .catch(() => {
        if (!cancelled) setLoadError("読み込みに失敗しました");
      });
    return () => {
      cancelled = true;
    };
  }, [token]);

  useEffect(() => {
    if (!token) return;
    let cancelled = false;
    apiFetch<TicketSummary>("/v1/me/coaching/tickets", { token }).then((data) => {
      if (!cancelled) { setTicketSummary(data); if (data.balance.available > 0) setMethod("tickets"); }
    }).catch(() => { if (!cancelled) setTicketSummary(null); });
    return () => { cancelled = true; };
  }, [token]);

  // 枠と種目を選ぶと、返ってくる目安（送った翌日の12時）と、いま受付中のコーチがいるかを出す。
  useEffect(() => {
    if (!token || !me?.demo) return;
    let cancelled = false;
    apiFetch<ReviewEstimate>(`/v1/coaching/estimate?tier=${tier}&event=${encodeURIComponent(event)}`, { token })
      .then((d) => {
        if (!cancelled) setEstimate(d);
      })
      .catch(() => {
        if (!cancelled) setEstimate(null);
      });
    return () => {
      cancelled = true;
    };
  }, [token, tier, event, me?.demo]);

  async function onPickFile(picked: File | null) {
    setError("");
    setFile(picked);
    if (!picked) return;
    if (tooLong(await videoDuration(picked))) {
      setError(`動画は${maxReviewSeconds}秒以内にしてください`);
      setFile(null);
    }
  }

  async function onSubmit(e: FormEvent) {
    e.preventDefault();
    if (!question.trim()) {
      setError("相談内容を記入してください");
      return;
    }
    if (!file) {
      setError("動画を選んでください");
      return;
    }
    if (file.size > maxBytes) {
      setError("100MB以内の動画にしてください");
      return;
    }
    if (method === "tickets" && (ticketSummary?.balance.available ?? 0) < 2) {
      setError("チケットが不足しています。先に購入してから動画を選んでください。");
      return;
    }
    if (!acquire()) return;
    setError("");
    try {
      const signed = await apiFetch<CloudinarySignature>("/v1/uploads/coaching-video", { method: "POST", token });
      setUploadPhase("upload");
      setUpload({ loaded: 0, total: file.size, percent: 0 });
      const payload = await uploadToCloudinary("video", file, signed, setUpload);
      if (payload.public_id !== signed.public_id) throw new Error("アップロードに失敗しました");
      setUploadPhase("saving");
      const created = await apiFetch<ReviewRequest>("/v1/coaching/requests", {
        method: "POST",
        token,
        body: JSON.stringify({ tier, event, question: question.trim(), video_public_id: payload.public_id, method }),
      });
      if (created.checkout_url) {
        window.location.href = created.checkout_url;
        return;
      }
      router.push(`/coaching/requests/${created.id}`);
    } catch (err) {
      setError(apiErrorText(err, err instanceof Error ? err.message : "送信に失敗しました"));
      setUpload(null);
      release();
    }
  }

  if (loading || !session || (!me && !loadError)) {
    return <p className="text-sm text-slate-200">読み込み中...</p>;
  }
  if (!me) {
    return <p className="text-sm text-red-400">{loadError}</p>;
  }

  return (
    <div className="mx-auto max-w-md space-y-6">
      <div className="space-y-2">
        {estimate?.payments ? null : (
          <p className="inline-flex rounded-full border border-blue-400/40 bg-blue-500/15 px-2.5 py-0.5 text-xs font-semibold text-blue-300">
            デモ版
          </p>
        )}
        <h1 className="text-2xl font-bold">フォーム添削を頼む</h1>
        <Link href="/coaches" className="flex min-h-12 items-center justify-between gap-3 rounded-xl border border-white/20 bg-white/10 px-4 py-3 text-sm font-semibold text-slate-100 transition hover:border-blue-400/50 hover:bg-white/15 active:scale-[0.98]">
          コーチ一覧<ArrowRight className="size-4 text-blue-300" aria-hidden />
        </Link>
      </div>

      {!me.demo ? (
        <DemoCodeForm
          intro="フォーム添削は準備中です。いまはデモ版として、運営から招待コードを受け取った人だけが使えます。"
          token={token}
          onJoined={() => void loadMe()}
        />
      ) : !me.birth_month ? (
        <div className="space-y-3">
          <p className="text-sm text-slate-200">先に生まれた年月を登録してください。18歳未満の人は保護者の同意が必要です。</p>
          <Link
            href="/me/coaching"
            className="flex min-h-11 w-full items-center justify-center rounded-md bg-blue-600 px-4 text-sm font-semibold text-white"
          >
            生まれた年月を登録する
          </Link>
        </div>
      ) : (
        <form onSubmit={onSubmit} className="space-y-5">
          <fieldset className="space-y-2">
            <legend className="text-sm font-semibold">種目</legend>
            <div className="flex flex-wrap gap-2">
              {reviewEvents.map((value) => (
                <button key={value} type="button" aria-pressed={event === value} onClick={() => setEvent(value)} className={chip(event === value)}>
                  {coachEventLabels[value]}
                </button>
              ))}
            </div>
            <p className="text-xs text-slate-300">最初は100mだけです。コーチが増えしだい、種目を増やします。</p>
          </fieldset>

          <label className="block space-y-2 text-sm">
            <span className="font-semibold">相談内容</span>
            <Textarea value={question} onChange={(e) => setQuestion(e.target.value)} rows={4} maxLength={1000} required disabled={pending} placeholder="例：後半にスピードが落ちます。動画を見て、改善点や練習方法を教えてください。" />
            <span className="block text-xs text-slate-300">{question.length}/1000文字</span>
          </label>

          {ticketSummary ? <div className="space-y-2 rounded-xl border border-white/20 bg-white/5 p-3 text-sm">
            <p>チケット残高：{ticketSummary.balance.available}枚（確保中 {ticketSummary.balance.reserved}枚）</p>
            <p className="text-xs text-slate-300">この添削は2枚です。先に購入してから動画を選んでください。</p>
            <Link href="/me/coaching/tickets?return_to=%2Fcoaching%2Fnew" className="flex min-h-12 items-center justify-between gap-3 rounded-xl border border-white/20 bg-white/10 px-4 py-3 text-sm font-semibold text-slate-100 transition hover:border-blue-400/50 hover:bg-white/15 active:scale-[0.98]">
              チケットを購入・確認する<ArrowRight className="size-4 text-blue-300" aria-hidden />
            </Link>
          </div> : null}
          <label className="block space-y-1 text-sm">
            <span className="font-semibold">動画({maxReviewSeconds}秒まで)</span>
            <input
              className="block w-full text-sm text-slate-300 file:mr-3 file:rounded-full file:border-0 file:bg-white/15 file:px-3 file:py-2 file:text-sm file:font-semibold file:text-slate-100"
              type="file"
              accept="video/mp4,video/quicktime,video/webm"
              onChange={(e) => void onPickFile(e.target.files?.[0] ?? null)}
              disabled={pending}
            />
          </label>

          {upload ? (
            <UploadProgress noun="動画" phase={uploadPhase} progress={upload} fileName={file?.name} fileSize={file?.size} />
          ) : null}
          {error ? <p className="text-sm text-red-400">{error}</p> : null}
          {estimate ? (
            <p className="rounded-xl border border-blue-400/40 bg-blue-500/15 p-3 text-sm text-slate-100">
              返ってくる目安: {formatDateTime(estimate.due_at)}まで
              <span className="block text-xs text-slate-300">
                {estimate.coaches_on_duty
                  ? "夜に送っても、翌日の昼までに返ってきます。"
                  : "いま受付中のコーチはいません。誰かが受付を始めたら回ります。"}
              </span>
            </p>
          ) : null}
          {estimate?.payments || (ticketSummary?.balance.available ?? 0) > 0 ? (
            <fieldset className="space-y-2">
              <legend className="text-sm font-semibold">お支払い（1本 {formatYen(estimate?.price_yen ?? 1200)}）</legend>
              <div className="flex flex-wrap gap-2">
                {ticketSummary ? <button type="button" aria-pressed={method === "tickets"} onClick={() => setMethod("tickets")} className={chip(method === "tickets")}>チケット（2枚）</button> : null}
                {(estimate?.payments ? Object.keys(reviewPayMethodLabels) as ReviewPayMethod[] : []).map((m) => (
                  <button key={m} type="button" aria-pressed={method === m} onClick={() => setMethod(m)} className={chip(method === m)}>
                    {reviewPayMethodLabels[m]}
                  </button>
                ))}
              </div>
              <p className="text-xs text-slate-300">
                {method === "tickets" ? "依頼時にチケットを確保し、返答が届いた時点で消費します。期限内に返答がなければ自動で戻ります。" : method === "card"
                  ? "送るとお支払いの画面に進みます。カードは送った時点では仮押さえだけで、返ってきた時に確定します。目安までに返ってこなければ取り消すので、代金はかかりません。"
                  : "送るとお支払いの画面に進み、その場でお支払いが済みます。目安までに返ってこなければ全額を返金します。"}
              </p>
            </fieldset>
          ) : (
            <p className="text-xs text-slate-300">デモ版のため料金はかかりません。目安までに返ってこなかった依頼は期限切れになります。</p>
          )}
          <Button type="submit" className="w-full" disabled={pending || !file}>
            {pending ? "送っています" : method === "tickets" ? "チケット2枚を確保して送る" : estimate?.payments ? `送ってお支払いへ進む（${formatYen(estimate.price_yen)}）` : "送る"}
          </Button>
        </form>
      )}
    </div>
  );
}
