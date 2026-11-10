"use client";

import { WorkOrderTermsView } from "@/components/work-order-terms";

import { CoachInfoRequestForm, ReviewInfoHistory } from "@/components/review-info-request";
import { Button } from "@/components/ui/button";
import { apiFetch, ApiRequestError } from "@/lib/api";
import { coachingMessagingEnabled } from "@/lib/features";
import { formatDateTime, formatYen } from "@/lib/format";
import { t, type Locale } from "@/lib/i18n";
import { coachEventLabels, coachTierLabels } from "@/lib/coach-recruit";
import { reviewQuestionText, reviewVideosPurgedNote } from "@/lib/review";
import type { CoachRequests, CoachReviewRequest } from "@/lib/types";
import { useActionLock } from "@/lib/use-action-lock";
import Link from "next/link";
import { useCallback, useEffect, useState } from "react";

// 打診の残り分数。expires_at はサーバーが打診の作成時刻から決めた値（23:00–07:00 は停止）で、
// ここでは表示のためだけに現在時刻との差を出す。起点の再計算はしない。
export function minutesLeft(iso: string, now: number) {
  return Math.max(0, Math.ceil((new Date(iso).getTime() - now) / 60000));
}

function RequestSummary({ request, locale }: { request: CoachReviewRequest; locale: Locale }) {
  return (
    <div className="space-y-0.5">
      <p className="font-semibold text-white">
        {coachEventLabels[request.event]} · {coachTierLabels[request.tier]}
      </p>
      {request.requested_coach_id ? <p className="text-sm font-semibold text-blue-300">{t(locale, "coach.reviewing.picked", { amount: formatYen(request.coach_reward_yen ?? 0) })}</p> : null}
      <p className="whitespace-pre-wrap break-words text-sm text-slate-200">{t(locale, "coach.reviewing.question", { text: reviewQuestionText(request) })}</p>
      {request.status === "needs_info" ? null : <p className="text-xs text-slate-300">{t(locale, "coach.reviewing.due", { date: formatDateTime(request.due_at) })}</p>}
    </div>
  );
}

// 届いた打診の引き受け・見送り（view="offers"）か、添削中の依頼の一覧（view="reviewing"）。
// /coach/offers と /coach/reviewing で使う。どちらを出すかは view で一意に決まる。
export function CoachRequestsSection({ token, view, locale }: { token: string | null; view: "offers" | "reviewing"; locale: Locale }) {
  const [data, setData] = useState<CoachRequests | null>(null);
  const [error, setError] = useState("");
  // 読み込みの失敗は、引き受け・見送りの失敗と分ける（読み込めたら消える）。
  const [loadError, setLoadError] = useState("");
  const [now, setNow] = useState(() => Date.now());
  const [reload, setReload] = useState(0);
  const { pending, acquire, release } = useActionLock();

  const fetchRequests = useCallback(() => apiFetch<CoachRequests>("/v1/coach/requests", { token }), [token]);

  useEffect(() => {
    let cancelled = false;
    const load = () => {
      fetchRequests()
        .then((d) => {
          if (cancelled) return;
          setData(d);
          setNow(Date.now());
          setLoadError("");
        })
        .catch(() => {
          if (!cancelled) setLoadError(t(locale, "coach.offers.loadError"));
        });
    };
    load();
    // 打診の残り時間を出し直し、期限が過ぎた打診を消すため、30秒ごとに読み直す。
    const timer = window.setInterval(load, 30000);
    const refresh = () => {
      if (document.visibilityState === "visible") load();
    };
    document.addEventListener("visibilitychange", refresh);
    return () => {
      cancelled = true;
      window.clearInterval(timer);
      document.removeEventListener("visibilitychange", refresh);
    };
  }, [fetchRequests, reload, locale]);

  async function answer(offerID: string, action: "accept" | "decline") {
    if (!acquire()) return;
    setError("");
    try {
      await apiFetch(`/v1/coach/offers/${offerID}/${action}`, { method: "POST", token });
    } catch (err) {
      setError(err instanceof ApiRequestError ? err.message : t(locale, "common.sendFailed"));
    } finally {
      release();
      setReload((n) => n + 1);
    }
  }

  const shownError = error || loadError;
  if (!data) return shownError ? <p className="text-sm text-red-400">{shownError}</p> : null;

  if (view === "offers") {
    return (
      <section className="space-y-2" id="offers">
        <p className="text-sm text-slate-300">
          {t(locale, "coach.offers.note")}
        </p>
        {data.offers.length === 0 ? <p className="text-sm text-slate-300">{t(locale, "coach.offers.empty")}</p> : null}
        <ul className="space-y-2">
          {data.offers.map((offer) => (
            <li key={offer.id} className="space-y-3 rounded-xl border border-blue-400/40 bg-blue-500/15 p-3">
              <RequestSummary request={offer.request} locale={locale} />
              <p className="text-sm font-semibold text-white">{t(locale, "coach.offers.deadline", { date: formatDateTime(offer.expires_at), minutes: minutesLeft(offer.expires_at, now) })}</p>
              {offer.terms ? <WorkOrderTermsView terms={offer.terms} /> : null}
              {coachingMessagingEnabled && !offer.request.info_request ? (
                <CoachInfoRequestForm requestID={offer.request.id} token={token} onSent={() => setReload((n) => n + 1)} />
              ) : null}
              <div className="flex gap-2">
                <Button type="button" className="flex-1" disabled={pending} onClick={() => void answer(offer.id, "accept")}>
                  {t(locale, "coach.offers.accept")}
                </Button>
                <button
                  type="button"
                  disabled={pending}
                  onClick={() => void answer(offer.id, "decline")}
                  className="min-h-11 flex-1 rounded-md border border-white/25 bg-white/10 px-4 text-sm font-semibold text-slate-100 transition active:scale-[0.98] disabled:opacity-50"
                >
                  {t(locale, "coach.offers.decline")}
                </button>
              </div>
            </li>
          ))}
        </ul>
        {shownError ? <p className="text-sm text-red-400">{shownError}</p> : null}
      </section>
    );
  }

  return (
    <section className="space-y-2" id="reviewing">
      {data.reviewing.length === 0 ? <p className="text-sm text-slate-300">{t(locale, "coach.reviewing.empty")}</p> : null}
      <ul className="space-y-3">
        {data.reviewing.map((request) => (
          <li key={request.id} className="space-y-3 rounded-xl border border-white/20 glass bg-white/10 p-3">
            <RequestSummary request={request} locale={locale} />
            {request.info_request && request.status === "needs_info" ? <ReviewInfoHistory info={request.info_request} viewer="coach" /> : null}
            {request.video_url ? (
              <video
                src={request.video_url}
                poster={request.thumbnail_url || undefined}
                controls
                playsInline
                preload="metadata"
                className="w-full rounded-lg bg-black"
              />
            ) : request.videos_purged ? (
              <p className="text-xs text-slate-300">{reviewVideosPurgedNote}</p>
            ) : null}
            <Link
              href={`/coach/requests/${request.id}`}
              className="flex min-h-11 w-full items-center justify-center rounded-md bg-blue-600 px-4 text-sm font-semibold text-pure-white"
            >
              {request.status === "clarifying" ? t(locale, "coach.reviewing.writeFollowup") : request.status === "needs_info" ? t(locale, "coach.reviewing.open") : t(locale, "coach.reviewing.writeResponse")}
            </Link>
          </li>
        ))}
      </ul>
      {shownError ? <p className="text-sm text-red-400">{shownError}</p> : null}
    </section>
  );
}
