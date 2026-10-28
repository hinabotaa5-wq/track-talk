"use client";

import { PageLoading } from "@/components/page-loading";

import { CoachingConsultations } from "@/components/coaching-consultations";

import { useAuth } from "@/components/auth-provider";
import { reportConfirmOptions, useConfirm } from "@/components/confirm-dialog";
import { ReviewCoachCard, ReviewResponseView } from "@/components/review-response";
import { apiFetch, ApiRequestError } from "@/lib/api";
import { coachEventLabels, coachTierLabels } from "@/lib/coach-recruit";
import { coachingMessagingEnabled } from "@/lib/features";
import { formatDateTime, formatYen } from "@/lib/format";
import { helpfulLabels, reviewQuestionText, reviewPayMethodLabels, reviewStatusLabels } from "@/lib/review";
import type { HelpfulAnswer, ReviewPayment, ReviewPayMethod, ReviewRequest } from "@/lib/types";
import { useActionLock } from "@/lib/use-action-lock";
import Link from "next/link";
import { useParams, useRouter, useSearchParams } from "next/navigation";
import { Suspense, useEffect, useState } from "react";

const chip = (on: boolean) =>
  `rounded-full border px-3 py-1.5 text-sm transition active:scale-[0.98] disabled:opacity-50 ${
    on ? "border-blue-400 bg-blue-500/30 text-white" : "border-white/25 bg-white/10 text-slate-200"
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
      setError(err instanceof ApiRequestError ? err.message : "送信に失敗しました");
    } finally {
      release();
    }
  }

  async function report() {
    if (!(await confirmDialog(reportConfirmOptions({ title: "この返答を運営に通報しますか？" })))) return;
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
      else setError("通報できませんでした");
    } finally {
      release();
    }
  }

  return (
    <div className="space-y-5">
      {response.helpful_asked ? (
        <section className="space-y-2">
          <h2 className="text-sm font-semibold">アドバイスは役に立ちましたか</h2>
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
        <h2 className="text-sm font-semibold">この添削の評価</h2>
        <div className="flex gap-1" role="radiogroup" aria-label="評価">
          {[1, 2, 3, 4, 5].map((n) => (
            <button
              key={n}
              type="button"
              role="radio"
              aria-checked={stars === n}
              aria-label={`${n}つ星`}
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
          <h2 className="text-sm font-semibold">回答について確認する（1回無料）</h2>
          <p className="text-xs text-slate-300">返却から7日以内に、回答のわからなかった点を1回確認できます。新しい相談は下の「追加相談」から送れます。</p>
          <textarea aria-label="回答のわからなかった点" value={clarifyBody} onChange={(e) => setClarifyBody(e.target.value)} maxLength={2000} disabled={pending} className="w-full rounded-md border border-white/25 bg-white/10 p-2 text-sm" />
          <button type="button" disabled={pending || !clarifyBody.trim()} onClick={() => void post("clarify", { reason: "other", body: clarifyBody })} className={chip(false)}>確認質問を送る（無料）</button>
        </section>
      )}

      {error ? <p className="text-sm text-red-400">{error}</p> : null}
      {reported ? (
        <p className="text-xs text-slate-300">通報しました。運営が確認します。</p>
      ) : (
        <button type="button" onClick={() => void report()} disabled={pending} className="text-xs text-slate-300 underline">
          この返答を通報する
        </button>
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
      return `${price}を仮押さえしています（${reviewPayMethodLabels[p.method]}）。返ってきた時に確定し、目安までに返ってこなければ取り消します。`;
    case "captured":
      return `${price}をお支払い済みです（${reviewPayMethodLabels[p.method]}）。`;
    case "canceled":
      return `添削が成立しなかったため、${price}の仮押さえを取り消しました。代金はかかりません。`;
    case "refunded":
      return `添削が成立しなかったため、${price}を返金しました。`;
  }
}

// お支払いがまだの依頼。支払い方法を選んで、支払い画面を開き直す。
function PayAgain({ request, token }: { request: ReviewRequest; token: string | null }) {
  const [error, setError] = useState("");
  const { pending, acquire, release } = useActionLock();

  async function pay(method: ReviewPayMethod) {
    if (!acquire()) return;
    setError("");
    try {
      const d = await apiFetch<ReviewRequest>(`/v1/coaching/requests/${request.id}/checkout`, {
        method: "POST",
        token,
        body: JSON.stringify({ method }),
      });
      if (d.checkout_url) window.location.assign(d.checkout_url);
    } catch (err) {
      setError(err instanceof ApiRequestError ? err.message : "お支払いの画面を開けませんでした");
      release();
    }
  }

  return (
    <section className="space-y-2">
      <p className="text-sm text-slate-200">お支払いが済むと、コーチを探し始めます。2時間たってもお支払いがなければ、この依頼は消えます。</p>
      {(Object.keys(reviewPayMethodLabels) as ReviewPayMethod[]).map((m) => (
        <button
          key={m}
          type="button"
          disabled={pending}
          onClick={() => void pay(m)}
          className="min-h-11 w-full rounded-md bg-blue-600 px-4 text-sm font-semibold text-white transition active:scale-[0.98] disabled:opacity-50"
        >
          {reviewPayMethodLabels[m]}で払う
        </button>
      ))}
      {error ? <p className="text-sm text-red-400">{error}</p> : null}
    </section>
  );
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
    apiFetch<ReviewRequest>(`/v1/coaching/requests/${id}`, { token })
      .then((d) => {
        if (!cancelled) setRequest(d);
      })
      .catch((err) => {
        if (!cancelled) setError(err instanceof ApiRequestError && err.status === 404 ? "依頼が見つかりません" : "読み込みに失敗しました");
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
  if (loading || !session || !request) return <PageLoading label="読み込み中..." />;

  const waiting = request.status === "queued" || request.status === "offered" || request.status === "in_review";

  return (
    <div className="mx-auto max-w-md space-y-5">
      <div className="space-y-1">
        <h1 className="text-2xl font-bold">フォーム添削</h1>
        <p className="text-sm text-slate-300">送った日時 {formatDateTime(request.created_at)}</p>
      </div>

      <section className="space-y-1 rounded-xl border border-blue-400/40 bg-blue-500/15 p-3">
        <p className="text-sm font-semibold text-white">{request.status === "expired" && request.requested_coach_id ? "依頼は終了しました" : reviewStatusLabels[request.status]}</p>
        {waiting || request.status === "awaiting_payment" ? (
          <p className="text-sm text-slate-100">返ってくる目安: {formatDateTime(request.due_at)}まで</p>
        ) : null}
        {request.requested_coach_id ? <div className="rounded-xl border border-blue-400/40 bg-blue-500/10 p-3 text-sm">
          <p className="font-semibold">選んだコーチ：{request.requested_coach_name || "選択したコーチ"}</p>
          <p>料金{formatYen(request.total_price_yen ?? request.base_price_yen ?? 1500)}</p>
          {request.status === "expired" ? <Link href="/coaching/new" className="mt-1 inline-block text-blue-300 underline">別のコーチを選んで申し込む</Link> : null}
        </div> : null}
        {request.tickets ? <p className="text-sm text-slate-200">チケット{request.tickets.quantity}枚：{request.tickets.status === "reserved" ? "添削のために確保中" : request.tickets.status === "consumed" ? "利用済み" : "残高に返却済み"}</p> : null}
      {request.payment ? <p className="text-xs text-slate-300">{paymentText(request.payment)}</p> : null}
      </section>

      {request.status === "awaiting_payment" ? (
        waitingForPayment ? (
          <p className="text-sm text-slate-100">お支払いを確かめています。このままお待ちください。</p>
        ) : (
          <PayAgain request={request} token={token} />
        )
      ) : null}

      {request.video_url ? (
        <video
          src={request.video_url}
          poster={request.thumbnail_url || undefined}
          controls
          playsInline
          preload="metadata"
          className="w-full rounded-xl bg-black"
        />
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
          <dt className="text-xs text-slate-300">誰に見てもらうか</dt>
          <dd>{coachTierLabels[request.tier]}</dd>
        </div>
        <div>
          <dt className="text-xs text-slate-300">種目</dt>
          <dd>{coachEventLabels[request.event]}</dd>
        </div>
        <div>
          <dt className="text-xs text-slate-300">相談内容</dt>
          <dd className="whitespace-pre-wrap break-words">{reviewQuestionText(request)}</dd>
        </div>
      </dl>

      <Link href="/me/coaching/requests" className="block text-center text-sm text-blue-300 underline">
        依頼の一覧へ
      </Link>
    </div>
  );
}

export default function ReviewRequestPage() {
  return (
    <Suspense fallback={<PageLoading label="読み込み中..." />}>
      <ReviewRequestView />
    </Suspense>
  );
}
