"use client";
import { L } from "@/lib/i18n";

import { ContactNotice, useSolicitationGuard } from "@/components/contact-notice";
import { ReviewCoachPicker } from "@/components/review-coach-picker";
import { SellerNote } from "@/components/seller-note";
import { PageLoading } from "@/components/page-loading";

import { useAuth } from "@/components/auth-provider";
import { DemoCodeForm } from "@/components/coaching-demo";
import { MinorPurchaseDeclaration } from "@/components/minor-purchase-declaration";
import { ReviewPayNotice } from "@/components/review-pay-notice";
import { ArrowRight } from "lucide-react";
import { Textarea } from "@/components/ui/textarea";
import { Button } from "@/components/ui/button";
import { UploadProgress } from "@/components/upload-progress";
import { apiFetch, apiErrorText, ApiRequestError } from "@/lib/api";
import { useMinorDeclaration } from "@/lib/minor-purchase";
import { coachEventLabels } from "@/lib/coach-recruit";
import { eagerVideoQuery, uploadVideoToCloudinary, type CloudinarySignature, type CloudinaryUploadProgress } from "@/lib/cloudinary-upload";
import {
  maxReviewSeconds,
  reviewEvents,
  availableReviewPayMethods,
  reviewPayMethodLabels,
  tooLong,
  videoDuration,
} from "@/lib/review";
import type { TicketSummary } from "@/lib/coaching-tickets";
import { formatDateTime, formatYen } from "@/lib/format";
import type { CoachEvent, CoachingMe, ReviewCoachOption, ReviewEstimate, ReviewPayMethod, ReviewRequest } from "@/lib/types";
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
    on ? "border-blue-400 bg-blue-500/30 text-pure-white" : "border-white/25 bg-white/10 text-slate-200"
  }`;

export default function NewReviewRequestPage() {
  const { session, token, loading } = useAuth();
  const router = useRouter();
  const [me, setMe] = useState<CoachingMe | null>(null);
  const [loadError, setLoadError] = useState("");
  const tier = "coach";
  const [event, setEvent] = useState<CoachEvent>(reviewEvents[0]);
  const [question, setQuestion] = useState("");
  const guard = useSolicitationGuard();
  const [file, setFile] = useState<File | null>(null);
  const [error, setError] = useState("");
  // 18歳未満の購入のたびの宣言。毎回チェックなしから始める。
  const declaration = useMinorDeclaration();
  const declared = declaration.checked;
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
  // 18歳未満が自分で払うときだけ、宣言のチェックを出して必須にする。
  const needsDeclaration = !!me?.is_minor && cashPayment;

  useEffect(() => {
    if (!token || !me?.demo) return;
    let cancelled = false;
    apiFetch<{ coaches: ReviewCoachOption[] }>(`/v1/coaching/nominees?event=${encodeURIComponent(event)}`, { token })
      .then((data) => { if (!cancelled) setCoaches(data.coaches); })
      .catch(() => { if (!cancelled) setCoachesError(L("コーチの一覧を読み込めませんでした。ページを開き直してください。")); })
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
      setLoadError(L("読み込みに失敗しました"));
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
        if (!cancelled) setLoadError(L("読み込みに失敗しました"));
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

  // 枠と種目を選ぶと、返却期限（送った時刻から24時間後）と、いま受付中のコーチがいるかを出す。
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
      setError(L("動画は{maxReviewSeconds}秒以内にしてください", { maxReviewSeconds }));
      setFile(null);
    }
  }

  async function onSubmit(e: FormEvent) {
    e.preventDefault();
    if (!estimate) { setError(L("料金と受付状況を確認してから送信してください。")); return; }
    if (!coach) { setError(L("コーチを選んでください。")); return; }
    if (question.trim() && !guard.allow(question)) return;
    if (!question.trim()) {
      setError(L("相談内容を記入してください"));
      return;
    }
    if (!file) {
      setError(L("動画を選んでください"));
      return;
    }
    if (file.size > maxBytes) {
      setError(L("100MB以内の動画にしてください"));
      return;
    }
    if (method === "tickets" && (ticketSummary?.balance.available ?? 0) < 2) {
      setError(L("チケットが不足しています。先に購入してから動画を選んでください。"));
      return;
    }
    if (needsDeclaration && !declared) {
      setError(L("購入の前に、保護者の方に許可をもらったことを確認して、チェックを入れてください。"));
      return;
    }
    if (!acquire()) return;
    setError("");
    try {
      const signed = await apiFetch<CloudinarySignature>(`/v1/uploads/coaching-video${eagerVideoQuery}`, { method: "POST", token });
      setUploadPhase("upload");
      setUpload({ loaded: 0, total: file.size, percent: 0 });
      const payload = await uploadVideoToCloudinary(file, signed, token, setUpload);
      if (payload.public_id !== signed.public_id) throw new Error(L("アップロードに失敗しました"));
      setUploadPhase("saving");
      const created = await apiFetch<ReviewRequest>("/v1/coaching/requests", {
        method: "POST",
        token,
        body: JSON.stringify({ tier, event, question: question.trim(), video_public_id: payload.public_id,
          method, use_tickets: method === "tickets", requested_coach_id: coach.id, ...declaration.body(needsDeclaration) }),
      });
      if (created.checkout_url) {
        window.location.assign(created.checkout_url);
        return;
      }
      router.push(`/coaching/requests/${created.id}`);
    } catch (err) {
      setError(apiErrorText(err, err instanceof Error ? err.message : L("送信に失敗しました")));
      if (err instanceof ApiRequestError && ["nomination_changed", "validation_error"].includes(err.code)) {
        try { const data = await apiFetch<{ coaches: ReviewCoachOption[] }>(`/v1/coaching/nominees?event=${encodeURIComponent(event)}`, { token }); setCoaches(data.coaches); } catch { /* 元のエラーを表示して再読込を促す。 */ }
      }
      setUpload(null);
      release();
    }
  }

  if (loading || !session || (!me && !loadError)) {
    return <PageLoading screen label={L("読み込み中...")} />;
  }
  if (!me) {
    return <p className="text-sm text-red-400">{loadError}</p>;
  }

  return (
    <div className="mx-auto max-w-md space-y-6">
      <div className="space-y-2">
        {!estimate || estimate.payments ? null : (
          <p className="inline-flex rounded-full border border-blue-400/40 bg-blue-500/15 px-2.5 py-0.5 text-xs font-semibold text-blue-300">{L("デモ版")}</p>
        )}
        <h1 className="text-2xl font-bold">{L("動画添削を頼む")}</h1>
        <p className="text-sm text-slate-300">{L("コーチを選んで、動画を送ります。料金はどのコーチでも同じです。")}</p>
        {me.demo ? <SellerNote /> : null}
      </div>

      {!me.demo ? (
        <DemoCodeForm
          intro={L("フォーム添削は準備中です。いまはデモ版として、運営から招待コードを受け取った人だけが使えます。")}
          token={token}
          onJoined={() => void loadMe()}
        />
      ) : !me.birth_month ? (
        <div className="space-y-3">
          <p className="text-sm text-slate-200">{L("先に生まれた年月を登録してください。18歳未満の方は、購入する前に保護者の許可を得てください。")}</p>
          <Link
            href="/me/coaching"
            className="flex min-h-11 w-full items-center justify-center rounded-md bg-blue-600 px-4 text-sm font-semibold text-pure-white"
          >{L("生まれた年月を登録する")}</Link>
        </div>
      ) : (
        <form onSubmit={onSubmit} className="space-y-5">
          <fieldset disabled={pending} className="space-y-2">
            <legend className="text-sm font-semibold">{L("種目")}</legend>
            <div className="flex flex-wrap gap-2">
              {reviewEvents.map((value) => (
                <button key={value} type="button" aria-pressed={event === value} onClick={() => { if (value !== event) { setEvent(value); setCoachesLoading(true); setCoachesError(""); setCoachID(""); setCoaches([]); } }} className={chip(event === value)}>
                  {coachEventLabels[value]}
                </button>
              ))}
            </div>
            <p className="text-xs text-slate-300">{L("最初は100mだけです。コーチが増えしだい、種目を増やします。")}</p>
          </fieldset>

          <ReviewCoachPicker coaches={coaches} value={coachID} onChange={setCoachID} disabled={pending} error={coachesError} loading={coachesLoading} />
          <div className="rounded-xl border border-blue-400/40 bg-blue-500/10 p-3 text-sm">
            <p>{L("動画添削（動画1本）")}</p>
            <p className="mt-1 text-lg font-bold">{L("{v0}／回", { v0: formatYen(totalPrice) })}</p>
          </div>
          <label className="block space-y-2 text-sm">
            <span className="font-semibold">{L("相談内容")}</span>
            <ContactNotice />
            {guard.warning}
            <Textarea value={question} onChange={(e) => setQuestion(e.target.value)} rows={4} maxLength={1000} required disabled={pending} placeholder={L("例：後半にスピードが落ちます。動画を見て、改善点や練習方法を教えてください。")} />
            <span className="block text-xs text-slate-300">{L("{length}/1000文字", { length: question.length })}</span>
          </label>

          {ticketSummary && ticketsUsable ? <div className="space-y-2 rounded-xl border border-white/20 bg-white/5 p-3 text-sm">
            <p className="text-xs text-slate-300">{L("動画添削1本はチケット2枚です。")}</p>
            <Link href="/me/coaching/tickets?return_to=%2Fcoaching%2Fnew" className="flex min-h-12 items-center justify-between gap-3 rounded-xl border border-white/20 bg-white/10 px-4 py-3 text-sm font-semibold text-slate-100 transition hover:border-blue-400/50 hover:bg-white/15 active:scale-[0.98]">
              {ticketSummary.can_purchase ? L("チケットを購入・確認する") : L("コーチングチケット")}<ArrowRight className="size-4 text-blue-300" aria-hidden />
            </Link>
          </div> : null}
          <label className="block space-y-1 text-sm">
            <span className="font-semibold">{L("動画({maxReviewSeconds}秒まで)", { maxReviewSeconds })}</span>
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
          {!estimate ? <p role="status" className="text-sm text-slate-300">{L("料金と受付状況を確認しています。表示されない場合はページを開き直してください。")}</p> : null}
          {estimate ? (
            <p className="rounded-xl border border-blue-400/40 bg-blue-500/15 p-3 text-sm text-slate-100">{L("返却期限:")}{" "}{formatDateTime(estimate.due_at)}
              <span className="block text-xs text-slate-300">
                {coach ? L("選んだコーチが対応します。期限までに回答がなければ代金はかかりません。") : L("送ってから24時間以内に返ってきます。")}
              </span>
            </p>
          ) : null}
          {estimate?.payments || ticketsUsable ? (
            <fieldset disabled={pending} className="space-y-2">
              <legend className="text-sm font-semibold">{L("お支払い（合計 {v0}）", { v0: formatYen(totalPrice) })}</legend>
              <div className="flex flex-wrap gap-2">
                {ticketsUsable ? <button type="button" aria-pressed={method === "tickets"} onClick={() => setMethod("tickets")} className={chip(method === "tickets")}>{L("チケット（2枚）")}</button> : null}
                {(estimate?.payments ? availableReviewPayMethods : []).map((m) => (
                  <button key={m} type="button" aria-pressed={method === m} onClick={() => setMethod(m)} className={chip(method === m)}>
                    {reviewPayMethodLabels[m]}
                  </button>
                ))}
              </div>
              <p className="text-xs text-slate-300">
                {method === "tickets" ? L("依頼時にチケットを確保し、返答が届いた時点で消費します。期限内に返答がなければ自動で戻ります。") : method === "card"
                  ? L("送るとお支払いの画面に進みます。カードは送った時点では仮押さえだけで、返ってきた時に確定します。目安までに返ってこなければ取り消すので、代金はかかりません。")
                  : L("送るとお支払いの画面に進み、その場でお支払いが済みます。目安までに返ってこなければ全額を返金します。")}
              </p>
              {estimate?.payments ? <ReviewPayNotice /> : null}
            </fieldset>
          ) : (
            <p className="text-xs text-slate-300">{L("デモ版のため料金はかかりません。目安までに返ってこなかった依頼は期限切れになります。")}</p>
          )}
          {needsDeclaration ? <MinorPurchaseDeclaration checked={declared} onChange={declaration.setChecked} onShown={declaration.markShown} disabled={pending} /> : null}
          <Button type="submit" className="w-full" disabled={pending || !file || !estimate || !coach || (needsDeclaration && !declared)}>
            {pending ? L("送っています") : cashPayment ? L("送ってお支払いへ（{v0}）", { v0: formatYen(cashPrice) }) : method === "tickets" ? L("チケット2枚を確保して送る") : L("送る")}
          </Button>
        </form>
      )}
    </div>
  );
}
