"use client";
import { L } from "@/lib/i18n";

import { PageLoading } from "@/components/page-loading";

import { CoachingConsultations } from "@/components/coaching-consultations";

import { useAuth } from "@/components/auth-provider";
import { reportConfirmOptions, useConfirm } from "@/components/confirm-dialog";
import { ReviewInfoHistory, ResubmitForm } from "@/components/review-info-request";
import { ReviewWithdrawalNotice } from "@/components/review-withdraw";
import { ReviewCoachCard, ReviewResponseView } from "@/components/review-response";
import { MinorPurchaseDeclaration } from "@/components/minor-purchase-declaration";
import { ReviewPayNotice } from "@/components/review-pay-notice";
import { PushPermissionGuide } from "@/components/push-permission-guide";
import { apiFetch, ApiRequestError } from "@/lib/api";
import { coachEventLabels, coachTierLabels } from "@/lib/coach-recruit";
import { coachingMessagingEnabled } from "@/lib/features";
import { formatDateTime, formatYen } from "@/lib/format";
import { reviewPriceYen } from "@/lib/review-pricing";
import { useMinorDeclaration } from "@/lib/minor-purchase";
import { availableReviewPayMethods, helpfulLabels, reviewQuestionText, reviewPayMethodLabels, reviewStatusLabels, reviewVideosPurgedNote } from "@/lib/review";
import type { CoachingMe, HelpfulAnswer, ReviewPayment, ReviewPayMethod, ReviewRequest } from "@/lib/types";
import { useActionLock } from "@/lib/use-action-lock";
import Link from "next/link";
import { useParams, useRouter, useSearchParams } from "next/navigation";
import { Suspense, useEffect, useState } from "react";

const chip = (on: boolean) =>
  `rounded-full border px-3 py-1.5 text-sm transition active:scale-[0.98] disabled:opacity-50 ${
    on ? "border-blue-400 bg-blue-500/30 text-pure-white" : "border-white/25 bg-white/10 text-slate-200"
  }`;

// 返答が届いたあとの操作: ★の評価、「ここがわからなかった」（1回）、7日後の「役に立ちましたか」、通報。
function ResponseActions({ request, token, onChange }: { request: ReviewRequest; token: string | null; onChange: (r: ReviewRequest) => void }) {
  const [error, setError] = useState("");
  const [reported, setReported] = useState(false);
  const [clarifyBody, setClarifyBody] = useState("");
  const [now] = useState(() => Date.now());
  const { pending, acquire, release } = useActionLock();
  const confirmDialog = useConfirm();
  const response = request.response;
  if (!response || response.hidden) return null;
  const stars = request.rating?.stars ?? 0;
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

      <section className="space-y-2">
        <h2 className="text-sm font-semibold">{L("この添削の評価")}</h2>
        <div className="flex gap-1" role="radiogroup" aria-label={L("評価")}>
          {[1, 2, 3, 4, 5].map((n) => (
            <button
              key={n}
              type="button"
              role="radio"
              aria-checked={stars === n}
              aria-label={L("{n}つ星", { n })}
              disabled={pending}
              onClick={() => void post("rating", { stars: n })}
              className={`text-3xl leading-none transition active:scale-[0.98] disabled:opacity-50 ${n <= stars ? "text-amber-300" : "text-white/25"}`}
            >
              ★
            </button>
          ))}
        </div>
      </section>

      {!coachingMessagingEnabled || response.clarify_reason || !response.created_at || now >= new Date(response.created_at).getTime() + 7 * 86400000 ? null : (
        <section className="space-y-2">
          <h2 className="text-sm font-semibold">{L("回答について確認する（1回無料）")}</h2>
          <p className="text-xs text-slate-300">{L("返却から7日以内に、回答のわからなかった点を1回確認できます。新しい相談は下の「追加相談」から送れます。")}</p>
          <textarea aria-label={L("回答のわからなかった点")} value={clarifyBody} onChange={(e) => setClarifyBody(e.target.value)} maxLength={2000} disabled={pending} className="w-full rounded-md border border-white/25 bg-white/10 p-2 text-sm" />
          <button type="button" disabled={pending || !clarifyBody.trim()} onClick={() => void post("clarify", { reason: "other", body: clarifyBody })} className={chip(false)}>{L("確認質問を送る（無料）")}</button>
        </section>
      )}

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
  const [error, setError] = useState("");
  const [minor, setMinor] = useState<boolean | null>(null);
  const declaration = useMinorDeclaration();
  const declared = declaration.checked;
  const { pending, acquire, release } = useActionLock();

  useEffect(() => {
    if (!token) return;
    let cancelled = false;
    apiFetch<CoachingMe>("/v1/me/coaching", { token })
      .then((d) => { if (!cancelled) setMinor(d.is_minor); })
      .catch(() => { if (!cancelled) setMinor(false); });
    return () => { cancelled = true; };
  }, [token]);

  async function pay(method: ReviewPayMethod) {
    if (minor && !declared) {
      setError(L("購入の前に、保護者の方に許可をもらったことを確認して、チェックを入れてください。"));
      return;
    }
    if (!acquire()) return;
    setError("");
    try {
      const d = await apiFetch<ReviewRequest>(`/v1/coaching/requests/${request.id}/checkout`, {
        method: "POST",
        token,
        body: JSON.stringify({ method, ...declaration.body(!!minor) }),
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
      {minor ? <MinorPurchaseDeclaration checked={declared} onChange={declaration.setChecked} onShown={declaration.markShown} disabled={pending} /> : null}
      {availableReviewPayMethods.map((m) => (
        <button
          key={m}
          type="button"
          disabled={pending || minor === null || (minor && !declared)}
          onClick={() => void pay(m)}
          className="min-h-11 w-full rounded-md bg-blue-600 px-4 text-sm font-semibold text-pure-white transition active:scale-[0.98] disabled:opacity-50"
        >{L("{v0}で払う", { v0: reviewPayMethodLabels[m] })}</button>
      ))}
      <ReviewPayNotice />
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

  return (
    <div className="mx-auto max-w-md space-y-5">
      {offerPushGuide && token ? <ReviewCompletionPushGuide key={`${session.user.id}:${id}`} id={id} userId={session.user.id} token={token} /> : null}
      <div className="space-y-1">
        <h1 className="text-2xl font-bold">{L("フォーム添削")}</h1>
        <p className="text-sm text-slate-300">{L("送った日時 {v0}", { v0: formatDateTime(request.created_at) })}</p>
      </div>

      <section className="space-y-1 rounded-xl border border-blue-400/40 bg-blue-500/15 p-3">
        <p className="text-sm font-semibold text-white">{request.withdrawal ? L("コーチの都合で依頼は終了しました") : request.status === "expired" && request.requested_coach_id ? L("依頼は終了しました") : reviewStatusLabels[request.status]}</p>
        {waiting || request.status === "awaiting_payment" ? (
          <p className="text-sm text-slate-100">{L("返却期限: {v0}", { v0: formatDateTime(request.due_at) })}</p>
        ) : null}
        {request.status === "needs_info" && request.info_request ? (
          <p className="text-sm text-slate-100">{L("再提出の期限: {v0}", { v0: formatDateTime(request.info_request.resubmit_by) })}</p>
        ) : null}
        {request.requested_coach_id ? <div className="rounded-xl border border-blue-400/40 bg-blue-500/10 p-3 text-sm">
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
        </>
      ) : null}

      <CoachingConsultations rootID={request.id} kind="review" token={token} />

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
