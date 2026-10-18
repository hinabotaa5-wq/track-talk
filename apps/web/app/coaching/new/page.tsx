"use client";

import { useAuth } from "@/components/auth-provider";
import { DemoCodeForm } from "@/components/coaching-demo";
import { Button } from "@/components/ui/button";
import { UploadProgress } from "@/components/upload-progress";
import { apiFetch, ApiRequestError } from "@/lib/api";
import { coachEventLabels, coachTierLabels } from "@/lib/coach-recruit";
import { uploadToCloudinary, type CloudinarySignature, type CloudinaryUploadProgress } from "@/lib/cloudinary-upload";
import {
  maxReviewSeconds,
  reviewEvents,
  reviewFocusLabels,
  reviewFocusOptions,
  reviewPayMethodLabels,
  tooLong,
  videoDuration,
} from "@/lib/review";
import { formatDateTime, formatYen } from "@/lib/format";
import type { CoachEvent, CoachingMe, CoachTier, ReviewEstimate, ReviewFocus, ReviewPayMethod, ReviewRequest } from "@/lib/types";
import { useActionLock } from "@/lib/use-action-lock";
import Link from "next/link";
import { useRouter } from "next/navigation";
import { useEffect, useState, type FormEvent } from "react";

// フォーム添削の依頼を送る。いまは招待コードを入れた人だけが使える。
// Stripe の鍵があるときは、送るとそのまま支払い画面へ進み、支払いが済んだらコーチを探し始める。
// 鍵がない間は支払いなしのデモ版。

const maxBytes = 100 * 1024 * 1024;

const chip = (on: boolean) =>
  `rounded-full border px-3 py-1.5 text-sm transition active:scale-95 ${
    on ? "border-blue-400 bg-blue-500/30 text-white" : "border-white/25 bg-white/10 text-slate-200"
  }`;

export default function NewReviewRequestPage() {
  const { session, token, loading } = useAuth();
  const router = useRouter();
  const [me, setMe] = useState<CoachingMe | null>(null);
  const [loadError, setLoadError] = useState("");
  const [tier, setTier] = useState<CoachTier>("coach");
  const [event, setEvent] = useState<CoachEvent>(reviewEvents[0]);
  const [focus, setFocus] = useState<ReviewFocus[]>([]);
  const [file, setFile] = useState<File | null>(null);
  const [error, setError] = useState("");
  const [upload, setUpload] = useState<CloudinaryUploadProgress | null>(null);
  const [uploadPhase, setUploadPhase] = useState<"upload" | "saving">("upload");
  const [estimate, setEstimate] = useState<ReviewEstimate | null>(null);
  const [method, setMethod] = useState<ReviewPayMethod>("card");
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

  function toggleFocus(value: ReviewFocus) {
    setFocus((current) => (current.includes(value) ? current.filter((f) => f !== value) : [...current, value]));
  }

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
    if (focus.length === 0) {
      setError("見てほしい点を1つ以上選んでください");
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
        body: JSON.stringify({ tier, event, focus, video_public_id: payload.public_id, method }),
      });
      if (created.checkout_url) {
        window.location.href = created.checkout_url;
        return;
      }
      router.push(`/coaching/requests/${created.id}`);
    } catch (err) {
      if (err instanceof ApiRequestError) {
        const field = err.fields ? Object.values(err.fields)[0] : "";
        setError(field || err.message);
      } else {
        setError(err instanceof Error ? err.message : "送信に失敗しました");
      }
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
            <legend className="text-sm font-semibold">誰に見てもらうか</legend>
            <Link href="/coaches" className="text-xs text-blue-300 underline">
              コーチ一覧を見る
            </Link>
            <div className="flex flex-wrap gap-2">
              {(Object.keys(coachTierLabels) as CoachTier[]).map((t) => (
                <button key={t} type="button" aria-pressed={tier === t} onClick={() => setTier(t)} className={chip(tier === t)}>
                  {coachTierLabels[t]}
                </button>
              ))}
            </div>
          </fieldset>

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

          <fieldset className="space-y-2">
            <legend className="text-sm font-semibold">見てほしい点(複数可)</legend>
            <div className="flex flex-wrap gap-2">
              {reviewFocusOptions.map((value) => (
                <button
                  key={value}
                  type="button"
                  aria-pressed={focus.includes(value)}
                  onClick={() => toggleFocus(value)}
                  className={chip(focus.includes(value))}
                >
                  {reviewFocusLabels[value]}
                </button>
              ))}
            </div>
          </fieldset>

          <label className="block space-y-1 text-sm">
            <span className="font-semibold">動画({maxReviewSeconds}秒まで)</span>
            <input
              className="block w-full text-sm text-slate-300 file:mr-3 file:rounded-full file:border-0 file:bg-white/15 file:px-3 file:py-2 file:text-sm file:font-semibold file:text-slate-100"
              type="file"
              accept="video/mp4,video/quicktime,video/webm"
              onChange={(e) => void onPickFile(e.target.files?.[0] ?? null)}
              disabled={pending}
            />
            <span className="block text-xs text-slate-300">
              ユニフォームの学校名やゼッケンなど、個人がわかるものが写らないようにしてください。動画はコーチだけが見られます。
            </span>
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
          {estimate?.payments ? (
            <fieldset className="space-y-2">
              <legend className="text-sm font-semibold">お支払い（1本 {formatYen(estimate.price_yen)}）</legend>
              <div className="flex flex-wrap gap-2">
                {(Object.keys(reviewPayMethodLabels) as ReviewPayMethod[]).map((m) => (
                  <button key={m} type="button" aria-pressed={method === m} onClick={() => setMethod(m)} className={chip(method === m)}>
                    {reviewPayMethodLabels[m]}
                  </button>
                ))}
              </div>
              <p className="text-xs text-slate-300">
                {method === "card"
                  ? "送るとお支払いの画面に進みます。カードは送った時点では仮押さえだけで、返ってきた時に確定します。目安までに返ってこなければ取り消すので、代金はかかりません。"
                  : "送るとお支払いの画面に進み、その場でお支払いが済みます。目安までに返ってこなければ全額を返金します。"}
              </p>
            </fieldset>
          ) : (
            <p className="text-xs text-slate-300">デモ版のため料金はかかりません。目安までに返ってこなかった依頼は期限切れになります。</p>
          )}
          <Button type="submit" className="w-full" disabled={pending || !file}>
            {pending ? "送っています" : estimate?.payments ? `送ってお支払いへ進む（${formatYen(estimate.price_yen)}）` : "送る"}
          </Button>
        </form>
      )}
    </div>
  );
}
