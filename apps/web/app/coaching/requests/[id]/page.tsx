"use client";
import { LoadedReferralCard, ReferralInput, validReferralInput } from "@/components/coaching-referral";
import { L } from "@/lib/i18n";

import { PageLoading } from "@/components/page-loading";

import { useAuth } from "@/components/auth-provider";
import { reportConfirmOptions, useConfirm } from "@/components/confirm-dialog";
import { ReviewInfoHistory, ResubmitForm } from "@/components/review-info-request";
import { Button } from "@/components/ui/button";
import { ReviewWithdrawalNotice } from "@/components/review-withdraw";
import { CoachingConsultations } from "@/components/coaching-consultations";
import { ReviewCoachCard, ReviewResponseView } from "@/components/review-response";
import { MinorPurchaseDeclaration } from "@/components/minor-purchase-declaration";
import { PushPermissionGuide } from "@/components/push-permission-guide";
import { apiFetch, ApiRequestError } from "@/lib/api";
import { coachEventLabels, coachTierLabels } from "@/lib/coach-recruit";
import { formatDateTime, formatYen } from "@/lib/format";
import { reviewPriceYen } from "@/lib/review-pricing";
import { useMinorDeclaration } from "@/lib/minor-purchase";
import { availableReviewPayMethods, helpfulLabels, reviewQuestionText, reviewPayMethodLabels, reviewStatusLabels, reviewVideosPurgedNote } from "@/lib/review";
import type { HelpfulAnswer, ReviewPayment, ReviewPayMethod, ReviewRequest } from "@/lib/types";
import { useNow } from "@/lib/use-now";
import { useActionLock } from "@/lib/use-action-lock";
import Link from "next/link";
import { useParams, useRouter, useSearchParams } from "next/navigation";
import { Suspense, useEffect, useState } from "react";

const chip = (on: boolean) =>
  `rounded-full border px-3 py-1.5 text-sm transition active:scale-[0.98] disabled:opacity-50 ${
    on ? "border-blue-400 bg-blue-500/30 text-pure-white" : "border-white/25 bg-white/10 text-slate-200"
  }`;

// 返答が届いたあとの操作: 7日後の「役に立ちましたか」、通報。（星評価は廃止）
function ResponseActions({ request, token, onChange }: { request: ReviewRequest; token: string | null; onChange: (r: ReviewRequest) => void }) {
  const [error, setError] = useState("");
  const [reported, setReported] = useState(false);
  const { pending, acquire, release } = useActionLock();
  const confirmDialog = useConfirm();
  const response = request.response;
  if (!response || response.hidden) return null;
  const helpful = request.rating?.helpful ?? null;

  async function post(path: string, body: unknown) {
    if (!acquire()) return;
    setError("");
    try {
      onChange(await apiFetch<ReviewRequest>(`/v1/coaching/requests/${request.id}/${path}`, { method: "POST", token, body: JSON.stringify(body) }));
    } catch (err) {
      setError(err instanceof ApiRequestError ? err.message : L("送信に失敗しました"));
    } finally {
      release();
    }
  }

  async function report() {
    if (!(await confirmDialog(reportConfirmOptions({ title: L("この返答を運営に通報しますか？") })))) return;
    if (!acquire()) return;
    setError("");
    try {
      await apiFetch("/v1/reports", {
        method: "POST",
        token,
        body: JSON.stringify({ target_type: "review_response", target_id: request.id, reason: "other", detail: "" }),
      });
      setReported(true);
    } catch (err) {
      if (err instanceof ApiRequestError && err.status === 409) setReported(true);
      else setError(L("通報できませんでした"));
    } finally {
      release();
    }
  }

  return (
    <div className="space-y-5">
      {response.helpful_asked ? (
        <section className="space-y-2">
          <h2 className="text-sm font-semibold">{L("アドバイスは役に立ちましたか")}</h2>
          <div className="flex flex-wrap gap-2">
            {(Object.keys(helpfulLabels) as HelpfulAnswer[]).map((h) => (
              <button key={h} type="button" disabled={pending} aria-pressed={helpful === h} onClick={() => void post("helpful", { helpful: h })} className={chip(helpful === h)}>
                {helpfulLabels[h]}
              </button>
            ))}
          </div>
        </section>
      ) : null}

      {error ? <p className="text-sm text-red-400">{error}</p> : null}
      {reported ? (
        <p className="text-xs text-slate-300">{L("通報しました。運営が確認します。")}</p>
      ) : (
        <button type="button" onClick={() => void report()} disabled={pending} className="text-xs text-slate-300 underline">{L("この返答を通報する")}</button>
      )}
    </div>
  );
}

// 支払いのあと戻ってきたとき、Stripe からの知らせが届いてコーチを探し始めるまで、何回か読み直す。
const paidPolls = 10;

function paymentText(p: ReviewPayment): string {
  const price = formatYen(p.amount_yen);
  switch (p.status) {
    case "authorized":
      return L("{price}を仮押さえしています（{v1}）。返ってきた時に確定し、目安までに返ってこなければ取り消します。", { price, v1: reviewPayMethodLabels[p.method] });
    case "captured":
      return L("{price}をお支払い済みです（{v1}）。", { price, v1: reviewPayMethodLabels[p.method] });
    case "canceled":
      return L("添削が成立しなかったため、{price}の仮押さえを取り消しました。代金はかかりません。", { price });
    case "refunded":
      return L("添削が成立しなかったため、{price}を返金しました。", { price });
  }
}

// お支払いがまだの依頼。支払い方法を選んで、支払い画面を開き直す。
// 18歳未満は、開き直すときも購入のたびの宣言にチェックが要る。
function PayAgain({ request, token }: { request: ReviewRequest; token: string | null }) {
  const [referralCode, setReferralCode] = useState("");
  const [referralWarning, setReferralWarning] = useState(false);
  const [error, setError] = useState("");
  const declaration = useMinorDeclaration();
  const declared = declaration.checked;
  const { pending, acquire, release } = useActionLock();

  async function pay(method: ReviewPayMethod) {
    if (!declared) {
      setError(L("購入前の注意事項を確認して、チェックを入れてください。"));
      return;
    }
    if (!acquire()) return;
    setError("");
    try {
      if (!await validReferralInput(referralCode, token)) {
        setReferralCode("");
        setReferralWarning(true);
        release();
        return;
      }
      const d = await apiFetch<ReviewRequest>(`/v1/coaching/requests/${request.id}/checkout`, {
        method: "POST",
        token,
        body: JSON.stringify({ method, referral_code: referralCode, ...declaration.body(true) }),
      });
      if (d.checkout_url) window.location.assign(d.checkout_url);
    } catch (err) {
      setError(err instanceof ApiRequestError ? err.message : L("お支払いの画面を開けませんでした"));
      release();
    }
  }

  return (
    <section className="space-y-2">
      <p className="text-sm text-slate-200">{L("お支払いが済むと、コーチを探し始めます。2時間たってもお支払いがなければ、この依頼は消えます。")}</p>
      <MinorPurchaseDeclaration checked={declared} onChange={declaration.setChecked} onShown={declaration.markShown} disabled={pending} />
      <ReferralInput code={referralCode} warning={referralWarning} disabled={pending}
        onChange={(code) => { setReferralCode(code); setReferralWarning(false); }} />
      {availableReviewPayMethods.map((m) => (
        <button
          key={m}
          type="button"
          disabled={pending || !declared}
          onClick={() => void pay(m)}
          className="min-h-11 w-full rounded-md bg-blue-600 px-4 text-sm font-semibold text-pure-white transition active:scale-[0.98] disabled:opacity-50"
        >{L("{v0}で払う", { v0: reviewPayMethodLabels[m] })}</button>
      ))}
      {error ? <p className="text-sm text-red-400">{error}</p> : null}
    </section>
  );
}

function ReviewCompletionPushGuide({ id, userId, token }: { id: string; userId: string; token: string }) {
  const [available, setAvailable] = useState(false);

  useEffect(() => {
    let active = true;
    void Promise.all([
      apiFetch<{ settings: { category: string; push: boolean }[] }>("/v1/me/notification-settings", { token }),
      apiFetch<{ public_key: string }>("/v1/push/public-key"),
    ]).then(([settings, key]) => {
      if (active && key.public_key && settings.settings.some((setting) => setting.category === "review" && setting.push)) {
        setAvailable(true);
      }
    }).catch(() => {
      // 返却通知を使えることが確認できたときだけ案内する。
    });
    return () => { active = false; };
  }, [token]);

  return available ? <PushPermissionGuide userId={userId} token={token} completedReviewId={id} /> : null;
}

// 送った添削の依頼と、届いた返答。
function ReviewRequestView() {
  const { session, token, loading } = useAuth();
  const router = useRouter();
  const { id } = useParams<{ id: string }>();
  const paid = useSearchParams().get("paid") === "1";
  const [polls, setPolls] = useState(0);
  const [request, setRequest] = useState<ReviewRequest | null>(null);
  const [error, setError] = useState("");

  const [cancelError, setCancelError] = useState("");
  const now = useNow();
  const { pending: canceling, acquire: acquireCancel, release: releaseCancel } = useActionLock();
  const confirmDialog = useConfirm();

  async function cancelReview() {
    if (!await confirmDialog({
      title: L("添削依頼をキャンセルしますか？"),
      body: L("お支払いは全額返金・取消し、チケットは返還します。"),
      confirmLabel: L("依頼をキャンセルする"),
      cancelLabel: L("戻る"),
    }) || !acquireCancel()) return;
    setCancelError("");
    const path = `/v1/coaching/requests/${id}`;
    try {
      setRequest(await apiFetch<ReviewRequest>(`${path}/cancel`, { method: "POST", token }));
    } catch (err) {
      setCancelError(err instanceof ApiRequestError ? err.message : L("依頼をキャンセルできませんでした"));
      if (err instanceof ApiRequestError && err.status === 409) {
        const latest = await apiFetch<ReviewRequest>(path, { token }).catch(() => null);
        if (latest) setRequest(latest);
      }
    } finally {
      releaseCancel();
    }
  }

  useEffect(() => {
    if (!loading && !session) {
      router.replace(`/login?next=${encodeURIComponent(window.location.pathname)}`);
    }
  }, [loading, session, router]);

  useEffect(() => {
    if (!token || !id) return;
    let cancelled = false;
    // お支払い待ちの間は、支払い画面から戻った URL（?paid=1）だけで成否を決めず、Stripe の実際の状態をサーバーで確かめる。
    // 確かめられなかったときは、Webhook が届くのを待つ（読み直しは下の paidPolls）。
    async function read() {
      let d = await apiFetch<ReviewRequest>(`/v1/coaching/requests/${id}`, { token });
      if (d.status === "awaiting_payment") {
        try {
          const synced = await apiFetch<ReviewRequest>(`/v1/coaching/requests/${id}/payment/sync`, { method: "POST", token });
          if (synced.status !== "awaiting_payment") d = await apiFetch<ReviewRequest>(`/v1/coaching/requests/${id}`, { token });
        } catch { /* Webhook の到着を待つ。 */ }
      }
      return d;
    }
    read()
      .then((d) => {
        if (!cancelled) setRequest(d);
      })
      .catch((err) => {
        if (!cancelled) setError(err instanceof ApiRequestError && err.status === 404 ? L("依頼が見つかりません") : L("読み込みに失敗しました"));
      });
    return () => {
      cancelled = true;
    };
  }, [token, id, polls]);

  const waitingForPayment = paid && request?.status === "awaiting_payment" && polls < paidPolls;
  useEffect(() => {
    if (!waitingForPayment) return;
    const t = window.setTimeout(() => setPolls((n) => n + 1), 2000);
    return () => window.clearTimeout(t);
  }, [waitingForPayment, polls]);

  if (error) return <p className="text-sm text-red-400">{error}</p>;
  if (loading || !session || !request) return <PageLoading screen label={L("読み込み中...")} />;

  const waiting = request.status === "queued" || request.status === "offered" || request.status === "in_review";
  const completedPayment = request.payment?.status === "authorized" || request.payment?.status === "captured";
  const offerPushGuide = paid && request.id === id && completedPayment && (waiting || request.status === "needs_info");

  const active = request.status === "in_review" || request.status === "needs_info";
  const deadline = request.status === "needs_info" ? request.info_request?.resubmit_by : request.due_at;
  const canCancel = active && !request.started_at && !!deadline && now !== null && Date.parse(deadline) > now;

  return (
    <div className="mx-auto max-w-md space-y-5">
      {request.payment?.status === "captured" && request.payment.amount_yen === reviewPriceYen ? <LoadedReferralCard token={token} /> : null}
      {offerPushGuide && token ? <ReviewCompletionPushGuide key={`${session.user.id}:${id}`} id={id} userId={session.user.id} token={token} /> : null}
      <div className="space-y-1">
        <h1 className="text-2xl font-bold">{L("フォーム添削")}</h1>
        <p className="text-sm text-slate-300">{L("送った日時 {v0}", { v0: formatDateTime(request.created_at) })}</p>
      </div>

      <section className="space-y-1 rounded-xl border border-blue-400/40 bg-blue-500/15 p-3">
        <p className="text-sm font-semibold text-white">{request.canceled_at ? L("依頼をキャンセルしました") : request.withdrawal ? L("コーチが依頼を返却しました") : request.status === "expired" && request.requested_coach_id ? L("依頼は終了しました") : reviewStatusLabels[request.status]}</p>
        {request.status === "in_review" ? (
          <p className="text-sm text-slate-100">{L("返却期限: {v0}", { v0: formatDateTime(request.due_at) })}</p>
        ) : null}
        {request.status === "needs_info" && request.info_request ? (
          <p className="text-sm text-slate-100">{L("再提出の期限: {v0}", { v0: formatDateTime(request.info_request.resubmit_by) })}</p>
        ) : null}
        {active && request.started_at ? <p className="text-sm text-slate-100">{L("現在コーチが添削を行っています")}</p> : null}
        {canCancel ? <Button type="button" variant="secondary" disabled={canceling} onClick={() => void cancelReview()}>{L("依頼をキャンセルする")}</Button> : null}
        {cancelError ? <p role="alert" className="text-sm text-red-400">{cancelError}</p> : null}
        {request.requested_coach_id && !(request.response && request.coach) ? <div className="rounded-xl border border-blue-400/40 bg-blue-500/10 p-3 text-sm">
          <p className="font-semibold">{L("選んだコーチ：{v0}", { v0: request.requested_coach_name || L("選択したコーチ") })}</p>
          <p>{L("料金{v0}", { v0: formatYen(request.total_price_yen ?? request.base_price_yen ?? reviewPriceYen) })}</p>
          {request.status === "expired" ? <Link href="/coaching/new" className="mt-1 inline-block text-blue-300 underline">{L("別のコーチを選んで申し込む")}</Link> : null}
        </div> : null}
        {request.tickets ? <p className="text-sm text-slate-200">{L("チケット{quantity}枚：{v1}", { quantity: request.tickets.quantity, v1: request.tickets.status === "reserved" ? L("添削のために確保中") : request.tickets.status === "consumed" ? L("利用済み") : L("残高に返却済み") })}</p> : null}
      {request.payment ? <p className="text-xs text-slate-300">{paymentText(request.payment)}</p> : null}
      </section>

      {request.status === "awaiting_payment" ? (
        waitingForPayment ? (
          <p className="text-sm text-slate-100">{L("お支払いを確かめています。このままお待ちください。")}</p>
        ) : (
          <PayAgain request={request} token={token} />
        )
      ) : null}

      {request.withdrawal ? <ReviewWithdrawalNotice withdrawal={request.withdrawal} paid={Boolean(request.payment)} /> : null}
      {request.info_request ? <ReviewInfoHistory info={request.info_request} viewer="requester" /> : null}
      {request.status === "needs_info" ? <ResubmitForm request={request} token={token} onChange={setRequest} /> : null}

      {request.video_url ? (
        <video
          src={request.video_url}
          poster={request.thumbnail_url || undefined}
          controls
          playsInline
          preload="metadata"
          className="w-full rounded-xl bg-black"
        />
      ) : request.videos_purged ? (
        <p className="rounded-xl border border-white/20 bg-white/5 p-3 text-sm text-slate-200">{reviewVideosPurgedNote}</p>
      ) : null}

      {request.response ? (
        <>
          {request.coach ? <ReviewCoachCard coach={request.coach} /> : null}
          <ReviewResponseView response={request.response} />
          <ResponseActions request={request} token={token} onChange={setRequest} />
          {id ? <CoachingConsultations rootID={id} kind="review" token={token} /> : null}
        </>
      ) : null}

      <dl className="space-y-2 rounded-xl border border-white/20 glass bg-white/10 p-3 text-sm">
        <div>
          <dt className="text-xs text-slate-300">{L("誰に見てもらうか")}</dt>
          <dd>{coachTierLabels[request.tier]}</dd>
        </div>
        <div>
          <dt className="text-xs text-slate-300">{L("種目")}</dt>
          <dd>{coachEventLabels[request.event]}</dd>
        </div>
        <div>
          <dt className="text-xs text-slate-300">{L("相談内容")}</dt>
          <dd className="whitespace-pre-wrap break-words">{reviewQuestionText(request)}</dd>
        </div>
      </dl>

      <Link href="/me/coaching/requests" className="block text-center text-sm text-blue-300 underline">{L("依頼の一覧へ")}</Link>
    </div>
  );
}

export default function ReviewRequestPage() {
  return (
    <Suspense fallback={<PageLoading screen label={L("読み込み中...")} />}>
      <ReviewRequestView />
    </Suspense>
  );
}
