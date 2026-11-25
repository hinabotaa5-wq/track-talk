"use client";
import { LoadedReferralCard, ReferralInput, checkReferralInput, type ReferralWarning } from "@/components/coaching-referral";
import { L } from "@/lib/i18n";

import { Button } from "@/components/ui/button";
import { useHeaderBackOverride } from "@/components/header-back-override";
import { PageLoading } from "@/components/page-loading";

import { useAuth } from "@/components/auth-provider";
import { reportConfirmOptions, useConfirm } from "@/components/confirm-dialog";
import { ReviewInfoHistory, ResubmitForm } from "@/components/review-info-request";
import { ReviewWithdrawalNotice } from "@/components/review-withdraw";
import { CoachingConsultations } from "@/components/coaching-consultations";
import { coachText } from "@/lib/coaching-display";
import { ReviewResponseView } from "@/components/review-response";
import { MinorPurchaseDeclaration } from "@/components/minor-purchase-declaration";
import { PushPermissionGuide } from "@/components/push-permission-guide";
import { apiFetch, ApiRequestError } from "@/lib/api";
import { formatDateTime } from "@/lib/format";
import { coachingCoachListEnabled } from "@/lib/features";
import { reviewPriceYen } from "@/lib/review-pricing";
import { useMinorDeclaration } from "@/lib/minor-purchase";
import { availableReviewPayMethods, helpfulLabels, reviewQuestionText, reviewVideosPurgedNote } from "@/lib/review";
import type { HelpfulAnswer, ReviewPayMethod, ReviewRequest } from "@/lib/types";
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
  const [reported, setReported] = useState(request.response_reported ?? false);
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

// 完了した1回プランのあとの導線は「新しい動画で依頼する」だけ（本体 #984）。
// 継続プランは休眠させたので（本体 #1007）、一覧フラグの真偽にかかわらず「継続プランを見る」と「準備中」の表示は出さない。
function CompletedNextSteps({ coachID }: { coachID: string | null }) {
  const again = coachID ? `/coaching/new?coach=${encodeURIComponent(coachID)}` : "/coaching/new";
  return (
    <nav aria-label={L("次の依頼")} className="grid gap-2">
      <Link href={again} className="block min-h-11 w-full rounded-xl bg-blue-600 px-4 py-3 text-center text-sm font-semibold text-pure-white transition active:scale-[0.98]">{L("新しい動画で依頼する")}</Link>
    </nav>
  );
}

// 支払いのあと戻ってきたとき、Stripe からの知らせが届いてコーチを探し始めるまで、何回か読み直す。
const paidPolls = 10;

// お支払いがまだの依頼。支払い方法を選んで、支払い画面を開き直す。
// 18歳未満は、開き直すときも購入のたびの宣言にチェックが要る。
function PayAgain({ request, token }: { request: ReviewRequest; token: string | null }) {
  const [referralCode, setReferralCode] = useState("");
  const [referralWarning, setReferralWarning] = useState<ReferralWarning>(false);
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
      const referralCheck = await checkReferralInput(referralCode, token);
      if (referralCheck !== "ok") {
        setReferralCode("");
        setReferralWarning(referralCheck);
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
      <p className="text-sm text-slate-200">{L("2時間たってもお支払いがなければ、この依頼は消えます。")}</p>
      <MinorPurchaseDeclaration checked={declared} onChange={declaration.setChecked} onShown={declaration.markShown} disabled={pending} />
      <ReferralInput token={token} code={referralCode} warning={referralWarning} disabled={pending}
        onChange={(code) => { setReferralCode(code); setReferralWarning(false); }} />
      {availableReviewPayMethods.map((m) => (
        <button
          key={m}
          type="button"
          disabled={pending || !declared}
          onClick={() => void pay(m)}
          className="min-h-11 w-full rounded-md bg-blue-600 px-4 text-sm font-semibold text-pure-white transition active:scale-[0.98] disabled:opacity-50"
        >{L("確定してお支払いへ")}</button>
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
      title: L("スタンダードプランの依頼をキャンセルしますか？"),
      body: request?.status === "awaiting_payment"
        ? L("まだお支払いは済んでいません。取り消すと、この依頼ではお支払いができなくなります。")
        : L("お支払いは全額返金・取消し、チケットは返還します。"),
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

  const feedbackDestination = !loading && session && !error && request?.id === id && request.response &&
    (request.status === "returned" || request.status === "clarifying") && !request.feedback_submitted
    ? `/coaching/requests/${id}/feedback` : null;
  useHeaderBackOverride(`/coaching/requests/${id}`, feedbackDestination);

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
  const canCancel = request.status === "awaiting_payment" || (active && !request.started_at && !!deadline && now !== null && Date.parse(deadline) > now);

  return (
    <div className="mx-auto max-w-md space-y-5">
      {completedPayment && request.payment?.amount_yen === reviewPriceYen ? <LoadedReferralCard token={token} variant="compact" /> : null}
      {offerPushGuide && token ? <ReviewCompletionPushGuide key={`${session.user.id}:${id}`} id={id} userId={session.user.id} token={token} /> : null}
      <div className="flex items-start justify-between gap-3">
        <div className="min-w-0 space-y-1">
          <h1 className="text-2xl font-bold">{L("スタンダードプラン")}</h1>
          <p className="text-sm text-slate-300">{L("送った日時 {v0}", { v0: formatDateTime(request.created_at) })}</p>
          {request.status === "in_review" ? (
            <p className="text-sm text-slate-300">{L("返却期限: {v0}", { v0: formatDateTime(request.due_at) })}</p>
          ) : null}
        </div>
        {request.coach?.id && coachingCoachListEnabled ? (
          <Link href={`/coaches/${encodeURIComponent(request.coach.id)}`} className="inline-flex min-h-9 items-center rounded-full border border-white/25 bg-white/10 px-4 text-sm font-medium text-blue-300 transition active:scale-[0.98]">
            {L("このコーチのプロフィールを見る")}
          </Link>
        ) : null}
      </div>

      {canCancel ? <Button type="button" variant="secondary" disabled={canceling} onClick={() => void cancelReview()}>{L("依頼をキャンセルする")}</Button> : null}
      {cancelError ? <p role="alert" className="text-sm text-red-400">{cancelError}</p> : null}
      {request.requested_coach_id && !(request.response && request.coach) && request.status === "expired" ? <Link href="/coaching/new" className="inline-block text-sm text-blue-300 underline">{L("別のコーチを選んで申し込む")}</Link> : null}

      {request.status === "awaiting_payment" ? (
        waitingForPayment ? (
          <p className="text-sm text-slate-100">{L("お支払いを確かめています。このままお待ちください。")}</p>
        ) : (
          <PayAgain request={request} token={token} />
        )
      ) : null}

      {request.withdrawal ? <ReviewWithdrawalNotice withdrawal={request.withdrawal} paid={Boolean(request.payment)} /> : null}
      {request.info_request ? <ReviewInfoHistory info={request.info_request} viewer="requester" body={coachText(request.info_request.body, request.info_request_ja, { mine: false })} /> : null}
      {request.status === "needs_info" ? <ResubmitForm request={request} token={token} onChange={setRequest} /> : null}

      <section aria-label={L("依頼内容")} className="space-y-3 rounded-2xl border border-white/20 glass bg-white/5 p-4">
        <h2 className="text-sm font-semibold text-slate-300">{L("依頼内容")}</h2>
        {request.video_url ? (
          <video
            src={request.video_url}
            poster={request.thumbnail_url || undefined}
            controls
            playsInline
            preload="metadata"
            className="w-full max-w-[260px] max-h-[300px] rounded-xl bg-black"
          />
        ) : request.videos_purged ? (
          <p className="text-sm text-slate-200">{reviewVideosPurgedNote()}</p>
        ) : null}
        <div className="space-y-1">
          <h3 className="text-xs font-semibold text-slate-300">{L("相談内容")}</h3>
          <p className="whitespace-pre-wrap break-words text-sm">{reviewQuestionText(request)}</p>
        </div>
      </section>

      {request.response ? (
        <>
          <ReviewResponseView response={request.response} viewer="requester" coach={request.coach} />
          <ResponseActions request={request} token={token} onChange={setRequest} />
          {/* 追加質問は販売を終了した（本体 #984）。販売終了前に購入した分だけを記録として出す（新規の購入・入力欄は出さない）。 */}
          {id ? <CoachingConsultations rootID={id} kind="review" token={token} /> : null}
        </>
      ) : null}

      {request.status === "returned" ? <CompletedNextSteps coachID={request.coach?.id ?? null} /> : null}

      <Link href="/me/coaching/requests" className="block min-h-11 w-full rounded-xl border border-white/25 bg-white/10 px-4 py-3 text-center text-sm font-semibold text-blue-300 transition active:scale-[0.98]">{L("依頼の一覧へ")}</Link>
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
