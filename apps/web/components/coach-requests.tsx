"use client";

import { ReviewInfoHistory } from "@/components/review-info-request";
import { apiFetch } from "@/lib/api";
import { formatDateTime } from "@/lib/format";
import { t, type Locale } from "@/lib/i18n";
import { coachEventLabels, coachTierLabels } from "@/lib/coach-recruit";
import { reviewQuestionText, reviewVideosPurgedNote } from "@/lib/review";
import type { CoachRequests, CoachReviewRequest } from "@/lib/types";
import Link from "next/link";
import { useCallback, useEffect, useState } from "react";

function RequestSummary({ request, locale }: { request: CoachReviewRequest; locale: Locale }) {
  return (
    <div className="space-y-0.5">
      <p className="font-semibold text-white">
        {coachEventLabels[request.event]} · {coachTierLabels[request.tier]}
      </p>
      <p className="whitespace-pre-wrap break-words text-sm text-slate-200">{t(locale, "coach.reviewing.question", { text: reviewQuestionText(request) })}</p>
      {request.status === "needs_info" ? null : <p className="text-xs text-slate-300">{t(locale, "coach.reviewing.due", { date: formatDateTime(request.due_at) })}</p>}
    </div>
  );
}

export function CoachRequestsSection({ token, locale }: { token: string | null; locale: Locale }) {
  const [data, setData] = useState<CoachRequests | null>(null);
  const [loadError, setLoadError] = useState("");

  const fetchRequests = useCallback(() => apiFetch<CoachRequests>("/v1/coach/requests", { token }), [token]);

  useEffect(() => {
    let cancelled = false;
    const load = () => {
      fetchRequests()
        .then((d) => {
          if (cancelled) return;
          setData(d);
          setLoadError("");
        })
        .catch(() => {
          if (!cancelled) setLoadError(t(locale, "coach.offers.loadError"));
        });
    };
    load();
    // 新着・返却済みの依頼を一覧に反映する。
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
  }, [fetchRequests, locale]);

  if (!data) return loadError ? <p className="text-sm text-red-400">{loadError}</p> : null;

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
              {request.status === "clarifying" ? t(locale, "coach.reviewing.writeFollowup") : t(locale, "coach.reviewing.open")}
            </Link>
          </li>
        ))}
      </ul>
      {loadError ? <p className="text-sm text-red-400">{loadError}</p> : null}
    </section>
  );
}
