"use client";
import { L } from "@/lib/i18n";

import { PageLoading } from "@/components/page-loading";

import { CoachAvatar } from "@/components/coach-photo";
import { CoachEventChips, CoachEventEmpty, filterCoachesByEvent, type CoachEventFilter } from "@/components/coach-event-filter";
import { apiFetch } from "@/lib/api";
import { rememberCoachDetailPreview } from "@/lib/coach-detail-preview";
import { formatYen } from "@/lib/format";
import { coachEventLabels } from "@/lib/coach-recruit";
import type { PublicCoach } from "@/lib/types";
import Link from "next/link";
import { useEffect, useState } from "react";

// 公開中の非・動画添削プランがないコーチの価格行は出さない（「プラン未公開」の文言も代替文言も置かない）。
function coachPrice(coach: PublicCoach): string | null {
  const prices = (coach.plans ?? []).filter((plan) => plan.published && plan.service_type !== "video_review").map((plan) => plan.price_yen);
  if (prices.length === 0) return null;
  const minimum = Math.min(...prices);
  return `${formatYen(minimum)}${prices.some((price) => price !== minimum) ? L("〜") : ""}`;
}

export function CoachList() {
  const [result, setResult] = useState<{ coaches: PublicCoach[]; fetchedAt: number } | null>(null);
  const [error, setError] = useState("");
  const [event, setEvent] = useState<CoachEventFilter>("all");

  useEffect(() => {
    let cancelled = false;
    apiFetch<{ coaches: PublicCoach[] }>("/v1/coaches")
      .then((d) => {
        if (!cancelled) setResult({ coaches: d.coaches, fetchedAt: Date.now() });
      })
      .catch(() => {
        if (!cancelled) setError(L("読み込みに失敗しました"));
      });
    return () => {
      cancelled = true;
    };
  }, []);

  if (error) return <p role="alert" className="text-sm text-red-400">{error}</p>;
  if (!result) return <PageLoading screen label={L("読み込み中...")} />;
  // 並びは API の公平なローテーションのまま。種目で絞っても並べ替えない。
  const shown = filterCoachesByEvent(result.coaches, event);

  return (
    <div className="space-y-3">
      <CoachEventChips value={event} onChange={setEvent} />
      {shown.length === 0 && event === "all" ? <p className="text-sm text-slate-200">{L("まだいません")}</p> : null}
      {shown.length === 0 && event !== "all" ? <CoachEventEmpty onReset={() => setEvent("all")} /> : null}
      {shown.length > 0 ? (
        <ul className="grid grid-cols-2 gap-3">
          {shown.map((c) => {
            const price = coachPrice(c);
            return (
            <li id={`coach-${c.id}`} key={c.id} className="min-w-0">
              <Link href={`/coaches/${encodeURIComponent(c.id)}`} onNavigate={() => rememberCoachDetailPreview(c, result.fetchedAt)} className="glass flex h-full flex-col items-center gap-3 rounded-xl border border-white/20 bg-white/10 p-4 text-center text-sm transition-colors hover:border-blue-400/50 hover:bg-white/15 focus-visible:outline-2 focus-visible:outline-blue-400">
                <CoachAvatar url={c.photo_url} name={c.nickname} size={80} />
                <div className="w-full min-w-0 space-y-1 break-words">
                  <p className="font-bold text-white">{c.nickname ?? L("名前なし")}</p>
                  <p className="text-xs text-slate-200">{c.events.map((e) => coachEventLabels[e]).join(L("、"))}</p>
                  {c.hitokoto ? <p className="truncate text-xs text-slate-200">{c.hitokoto}</p> : null}
                  {price ? <p className="font-semibold text-blue-300">{price}</p> : null}
                </div>
              </Link>
            </li>
            );
          })}
        </ul>
      ) : null}
    </div>
  );
}
