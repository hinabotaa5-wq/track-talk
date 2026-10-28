"use client";

import { ReviewCoachPicker } from "@/components/review-coach-picker";
import { SellerNote } from "@/components/seller-note";
import { useAuth } from "@/components/auth-provider";
import { DemoCodeForm } from "@/components/coaching-demo";
import { GuardianAllowance } from "@/components/guardian-allowance";
import { ArrowRight } from "lucide-react";
import { Textarea } from "@/components/ui/textarea";
import { Button } from "@/components/ui/button";
import { UploadProgress } from "@/components/upload-progress";
import { apiFetch, apiErrorText, ApiRequestError } from "@/lib/api";
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
import type { CoachEvent, CoachingGuardian, CoachingMe, ReviewCoachOption, ReviewEstimate, ReviewPayMethod, ReviewRequest } from "@/lib/types";
import { useActionLock } from "@/lib/use-action-lock";
import Link from "next/link";
import { useRouter } from "next/navigation";
import { useEffect, useState, type FormEvent } from "react";

// フォーム添削の依頼を送る。いまは招待コードを入れた人だけが使える。
// 依頼するコーチは選手が選ぶ（運営からの割り当てはしない）。料金はどのコーチでも同じ。
// Stripe の鍵があるときは、送るとそのまま支払い画面へ進み、支払いが済んだら選んだコーチに依頼が届く。
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
  // 18歳未満で、保護者の同意や上限のために買えなかったとき。保護者の同意の画面への案内を出す。
  const [guardianBlocked, setGuardianBlocked] = useState(false);
  const [resendMessage, setResendMessage] = useState("");
  const [upload, setUpload] = useState<CloudinaryUploadProgress | null>(null);
  const [uploadPhase, setUploadPhase] = useState<"upload" | "saving">("upload");
  const [estimate, setEstimate] = useState<ReviewEstimate | null>(null);
  const [ticketSummary, setTicketSummary] = useState<TicketSummary | null>(null);
  const [method, setMethod] = useState<ReviewPayMethod | "tickets">("card");
  const { pending, acquire, release } = useActionLock();
  const [coaches, setCoaches] = useState<ReviewCoachOption[]>([]);
  const [coachID, setCoachID] = useState("");
  const [coachesLoading, setCoachesLoading] = useState(true);
  const [coachesError, setCoachesError] = useState("");
  const coach = coaches.find((c) => c.id === coachID);
  const totalPrice = estimate?.price_yen ?? 1500;
  const cashPrice = method === "tickets" ? 0 : totalPrice;
  // チケットの新規販売は止めた。購入済みのチケットが添削1本分（2枚）残っている人にだけ、チケットで払う選択肢を出す。
  const ticketsUsable = (ticketSummary?.balance.available ?? 0) >= 2;
  const cashPayment = !!estimate?.payments && cashPrice > 0;

  useEffect(() => {
    if (!token || !me?.demo) return;
    let cancelled = false;
    apiFetch<{ coaches: ReviewCoachOption[] }>(`/v1/coaching/nominees?event=${encodeURIComponent(event)}`, { token })
      .then((data) => { if (!cancelled) setCoaches(data.coaches); })
      .catch(() => { if (!cancelled) setCoachesError("コーチの一覧を読み込めませんでした。ページを開き直してください。"); })
      .finally(() => { if (!cancelled) setCoachesLoading(false); });
    return () => { cancelled = true; };
  }, [event, token, me?.demo]);

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
      if (!cancelled) { setTicketSummary(data); if (data.balance.available >= 2) setMethod("tickets"); }
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
    if (!estimate) { setError("料金と受付状況を確認してから送信してください。"); return; }
    if (!coach) { setError("コーチを選んでください。"); return; }
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
    setGuardianBlocked(false);
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
        body: JSON.stringify({ tier, event, question: question.trim(), video_public_id: payload.public_id,
          method, use_tickets: method === "tickets", requested_coach_id: coach.id }),
      });
      if (created.checkout_url) {
        window.location.assign(created.checkout_url);
        return;
      }
      router.push(`/coaching/requests/${created.id}`);
    } catch (err) {
      setError(apiErrorText(err, err instanceof Error ? err.message : "送信に失敗しました"));
      if (err instanceof ApiRequestError && err.code.startsWith("guardian_")) setGuardianBlocked(true);
      if (err instanceof ApiRequestError && ["nomination_changed", "validation_error"].includes(err.code)) {
        try { const data = await apiFetch<{ coaches: ReviewCoachOption[] }>(`/v1/coaching/nominees?event=${encodeURIComponent(event)}`, { token }); setCoaches(data.coaches); } catch { /* 元のエラーを表示して再読込を促す。 */ }
      }
      setUpload(null);
      release();
    }
  }

  // 保護者がまだ答えていないときに、前と同じアドレスへ同意のお願いを送り直す（前のリンクは使えなくなる）。
  async function onResendGuardianLink() {
    if (!acquire()) return;
    setResendMessage("");
    try {
      const guardian = await apiFetch<CoachingGuardian>("/v1/me/guardian-link", { method: "POST", token, body: JSON.stringify({}) });
      setMe((prev) => (prev ? { ...prev, guardian } : prev));
      setResendMessage("保護者の方に、同意のお願いメールを送り直しました。");
    } catch (err) {
      setResendMessage(apiErrorText(err, "メールを送れませんでした"));
    } finally {
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
        {!estimate || estimate.payments ? null : (
          <p className="inline-flex rounded-full border border-blue-400/40 bg-blue-500/15 px-2.5 py-0.5 text-xs font-semibold text-blue-300">
            デモ版
          </p>
        )}
        <h1 className="text-2xl font-bold">動画添削を頼む</h1>
        <p className="text-sm text-slate-300">コーチを選んで、動画を送ります。料金はどのコーチでも同じです。</p>
        <SellerNote />

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
      ) : me.is_minor && me.guardian?.status !== "consented" ? (
        // 文面は運営者の確定版。API の minorSelfPayBlock のエラー文と合わせる。
        <div className="space-y-3 rounded-2xl border border-amber-400/40 bg-amber-500/10 p-4 text-sm">
          <p className="text-slate-200">
            {me.guardian?.status === "declined"
              ? "保護者の方の同意が得られなかったため、有料サービスは利用できません。"
              : me.guardian?.status === "revoked"
                ? "保護者の方の同意が取り消されたため、有料サービスは利用できません。再度利用するには、保護者の方の同意が必要です。"
                : me.guardian?.status === "pending"
                  ? "有料サービスの利用には、保護者の方の同意が必要です。保護者の方に同意のお願いメールを送っています。同意が完了すると、ここから依頼できるようになります。"
                  : "有料サービスの利用には、保護者の方の同意が必要です。保護者の方に同意のお願いメールを送ってください。同意が完了すると、ここから依頼できるようになります。"}
          </p>
          {me.guardian?.status === "pending" ? (
            <Button type="button" className="w-full" disabled={pending} onClick={() => void onResendGuardianLink()}>
              同意のお願いを再送する
            </Button>
          ) : !me.guardian ? (
            <Link href="/me/coaching" className="flex min-h-11 w-full items-center justify-center rounded-md bg-blue-600 px-4 text-sm font-semibold text-white">
              保護者にメールを送る
            </Link>
          ) : null}
          {resendMessage ? <p role="status" className="text-slate-200">{resendMessage}</p> : null}
        </div>
      ) : (
        <form onSubmit={onSubmit} className="space-y-5">
          <fieldset disabled={pending} className="space-y-2">
            <legend className="text-sm font-semibold">種目</legend>
            <div className="flex flex-wrap gap-2">
              {reviewEvents.map((value) => (
                <button key={value} type="button" aria-pressed={event === value} onClick={() => { if (value !== event) { setEvent(value); setCoachesLoading(true); setCoachesError(""); setCoachID(""); setCoaches([]); } }} className={chip(event === value)}>
                  {coachEventLabels[value]}
                </button>
              ))}
            </div>
            <p className="text-xs text-slate-300">最初は100mだけです。コーチが増えしだい、種目を増やします。</p>
          </fieldset>

          <ReviewCoachPicker coaches={coaches} value={coachID} onChange={setCoachID} disabled={pending} error={coachesError} loading={coachesLoading} />
          <div className="rounded-xl border border-blue-400/40 bg-blue-500/10 p-3 text-sm">
            <p>動画添削（動画1本）</p>
            <p className="mt-1 text-lg font-bold">{formatYen(totalPrice)}／回</p>
          </div>
          <label className="block space-y-2 text-sm">
            <span className="font-semibold">相談内容</span>
            <Textarea value={question} onChange={(e) => setQuestion(e.target.value)} rows={4} maxLength={1000} required disabled={pending} placeholder="例：後半にスピードが落ちます。動画を見て、改善点や練習方法を教えてください。" />
            <span className="block text-xs text-slate-300">{question.length}/1000文字</span>
          </label>

          {ticketSummary && ticketsUsable ? <div className="space-y-2 rounded-xl border border-white/20 bg-white/5 p-3 text-sm">
            <p>チケット残高：{ticketSummary.balance.available}枚（確保中 {ticketSummary.balance.reserved}枚）</p>
            <p className="text-xs text-slate-300">動画添削1本はチケット2枚です。</p>
            <Link href="/me/coaching/tickets?return_to=%2Fcoaching%2Fnew" className="flex min-h-12 items-center justify-between gap-3 rounded-xl border border-white/20 bg-white/10 px-4 py-3 text-sm font-semibold text-slate-100 transition hover:border-blue-400/50 hover:bg-white/15 active:scale-[0.98]">
              {ticketSummary.can_purchase ? "チケットを購入・確認する" : "チケットの残高・履歴を見る"}<ArrowRight className="size-4 text-blue-300" aria-hidden />
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
          {guardianBlocked ? <Link href="/me/coaching" className="block text-sm text-blue-300 underline">保護者の同意・上限の状況を見る</Link> : null}
          {me.is_minor && me.guardian?.status === "consented" ? (
            <GuardianAllowance guardian={me.guardian} className="space-y-1 text-xs text-slate-300" />
          ) : null}
          {!estimate ? <p role="status" className="text-sm text-slate-300">料金と受付状況を確認しています。表示されない場合はページを開き直してください。</p> : null}
          {estimate ? (
            <p className="rounded-xl border border-blue-400/40 bg-blue-500/15 p-3 text-sm text-slate-100">
              返ってくる目安: {formatDateTime(estimate.due_at)}まで
              <span className="block text-xs text-slate-300">
                {coach ? "選んだコーチが対応します。期限までに回答がなければ代金はかかりません。" : "夜に送っても、翌日の昼までに返ってきます。"}
              </span>
            </p>
          ) : null}
          {estimate?.payments || ticketsUsable ? (
            <fieldset disabled={pending} className="space-y-2">
              <legend className="text-sm font-semibold">お支払い（合計 {formatYen(totalPrice)}）</legend>
              <div className="flex flex-wrap gap-2">
                {ticketsUsable ? <button type="button" aria-pressed={method === "tickets"} onClick={() => setMethod("tickets")} className={chip(method === "tickets")}>チケット（2枚）</button> : null}
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
          <Button type="submit" className="w-full" disabled={pending || !file || !estimate || !coach}>
            {pending ? "送っています" : cashPayment ? `送ってお支払いへ（${formatYen(cashPrice)}）` : method === "tickets" ? "チケット2枚を確保して送る" : "送る"}
          </Button>
        </form>
      )}
    </div>
  );
}
