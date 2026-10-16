"use client";

import { useAuth } from "@/components/auth-provider";
import { ReviewCoachCard, ReviewResponseView } from "@/components/review-response";
import { apiFetch, ApiRequestError } from "@/lib/api";
import { coachEventLabels, coachTierLabels } from "@/lib/coach-recruit";
import { formatDateTime } from "@/lib/format";
import { clarifyReasonLabels, helpfulLabels, reviewFocusLabels, reviewStatusLabels } from "@/lib/review";
import type { ClarifyReason, HelpfulAnswer, ReviewRequest } from "@/lib/types";
import { useActionLock } from "@/lib/use-action-lock";
import Link from "next/link";
import { useParams, useRouter } from "next/navigation";
import { useEffect, useState } from "react";

const chip = (on: boolean) =>
  `rounded-full border px-3 py-1.5 text-sm transition active:scale-95 disabled:opacity-50 ${
    on ? "border-blue-400 bg-blue-500/30 text-white" : "border-white/25 bg-white/10 text-slate-200"
  }`;

// 返答が届いたあとの操作: ★の評価、「ここがわからなかった」（1回）、7日後の「役に立ちましたか」、通報。
function ResponseActions({ request, token, onChange }: { request: ReviewRequest; token: string | null; onChange: (r: ReviewRequest) => void }) {
  const [error, setError] = useState("");
  const [reported, setReported] = useState(false);
  const { pending, acquire, release } = useActionLock();
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
    if (!window.confirm("この返答を運営に通報しますか？")) return;
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
              className={`text-3xl leading-none transition active:scale-90 disabled:opacity-50 ${n <= stars ? "text-amber-300" : "text-white/25"}`}
            >
              ★
            </button>
          ))}
        </div>
      </section>

      {response.clarify_reason ? null : (
        <section className="space-y-2">
          <h2 className="text-sm font-semibold">ここがわからなかった</h2>
          <p className="text-xs text-slate-300">選ぶと、コーチが1回だけ補足を送ってくれます。押せるのは1回だけです。</p>
          <div className="flex flex-wrap gap-2">
            {(Object.keys(clarifyReasonLabels) as ClarifyReason[]).map((r) => (
              <button
                key={r}
                type="button"
                disabled={pending}
                onClick={() => {
                  if (window.confirm(`「${clarifyReasonLabels[r]}」でコーチに補足を頼みますか？`)) void post("clarify", { reason: r });
                }}
                className={chip(false)}
              >
                {clarifyReasonLabels[r]}
              </button>
            ))}
          </div>
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

// 送った添削の依頼と、届いた返答。
export default function ReviewRequestPage() {
  const { session, token, loading } = useAuth();
  const router = useRouter();
  const { id } = useParams<{ id: string }>();
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
  }, [token, id]);

  if (error) return <p className="text-sm text-red-400">{error}</p>;
  if (loading || !session || !request) return <p className="text-sm text-slate-200">読み込み中...</p>;

  const waiting = request.status === "queued" || request.status === "offered" || request.status === "in_review";

  return (
    <div className="mx-auto max-w-md space-y-5">
      <div className="space-y-1">
        <h1 className="text-2xl font-bold">フォーム添削</h1>
        <p className="text-sm text-slate-300">送った日時 {formatDateTime(request.created_at)}</p>
      </div>

      <section className="space-y-1 rounded-xl border border-blue-400/40 bg-blue-500/15 p-3">
        <p className="text-sm font-semibold text-white">{reviewStatusLabels[request.status]}</p>
        {waiting ? <p className="text-sm text-slate-100">返ってくる目安: {formatDateTime(request.due_at)}まで</p> : null}
      </section>

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
          <dt className="text-xs text-slate-300">見てほしい点</dt>
          <dd>{request.focus.map((f) => reviewFocusLabels[f]).join("、")}</dd>
        </div>
      </dl>

      <Link href="/me/coaching/requests" className="block text-center text-sm text-blue-300 underline">
        依頼の一覧へ
      </Link>
    </div>
  );
}
