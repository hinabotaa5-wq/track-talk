"use client";
import { L } from "@/lib/i18n";

import { PageLoading } from "@/components/page-loading";

import { CoachAvatar } from "@/components/coach-photo";
import { apiFetch } from "@/lib/api";
import { rememberCoachDetailPreview } from "@/lib/coach-detail-preview";
import { formatYen } from "@/lib/format";
import { coachEventLabels } from "@/lib/coach-recruit";
import { coachDisplayText, countryFlagEmoji } from "@/lib/coach-display";
import type { PublicCoach } from "@/lib/types";
import Link from "next/link";
import { useEffect, useState } from "react";

// スタンダードの一覧なので、価格はコーチのいま有効なスタンダード価格を出す（本体 #1052）。
// 古い API（review_price_yen が無い）では従来どおり、公開中の非・動画添削プランの価格を出し、なければ価格行を出さない。
function coachPrice(coach: PublicCoach): string | null {
  if (typeof coach.review_price_yen === "number") return formatYen(coach.review_price_yen);
  const prices = (coach.plans ?? []).filter((plan) => plan.published && plan.service_type !== "video_review").map((plan) => plan.price_yen);
  if (prices.length === 0) return null;
  const minimum = Math.min(...prices);
  return `${formatYen(minimum)}${prices.some((price) => price !== minimum) ? L("〜") : ""}`;
}

export function CoachList({ onSettled }: { onSettled?: () => void }) {
  const [result, setResult] = useState<{ coaches: PublicCoach[]; fetchedAt: number } | null>(null);
  const [error, setError] = useState("");

  useEffect(() => {
    let cancelled = false;
    apiFetch<{ coaches: PublicCoach[] }>("/v1/coaches")
      .then((d) => {
        if (cancelled) return;
        setResult({ coaches: d.coaches, fetchedAt: Date.now() });
        onSettled?.();
      })
      .catch(() => {
        if (cancelled) return;
        setError(L("読み込みに失敗しました"));
        onSettled?.();
      });
    return () => {
      cancelled = true;
    };
    // 初回の取得だけを知らせる。以後の再取得はない。
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, []);

  if (error) return <p role="alert" className="text-sm text-red-400">{error}</p>;
  if (!result) return <PageLoading screen label={L("読み込み中...")} />;
  // 並びは API の公平なローテーションのまま。種目の絞り込みは置かない（本体 #991）。
  const shown = result.coaches;

  return (
    <div className="space-y-3">
      {shown.length === 0 ? <p className="text-sm text-slate-200">{L("まだいません")}</p> : null}
      {shown.length > 0 ? (
        <ul className="grid grid-cols-2 gap-3">
          {shown.map((c) => {
            const price = coachPrice(c);
            const displayName = coachDisplayText(c.nickname, c.nickname_en);
            const flag = countryFlagEmoji(c.country_code);
            return (
            <li id={`coach-${c.id}`} key={c.id} className="min-w-0">
              <Link href={`/coaches/${encodeURIComponent(c.id)}`} onNavigate={() => rememberCoachDetailPreview(c, result.fetchedAt)} className="app-card flex h-full flex-col items-center gap-3 border border-transparent p-4 text-center text-sm transition-all hover:border-blue-400/50 hover:shadow-md focus-visible:outline-2 focus-visible:outline-blue-400">
                <CoachAvatar url={c.photo_url} name={displayName} size={80} />
                <div className="w-full min-w-0 space-y-1 break-words">
                  <p className="font-bold text-white">
                    {flag ? <span aria-hidden="true" data-testid="coach-flag">{flag} </span> : null}
                    {displayName ?? L("名前なし")}
                  </p>
                  <p className="text-xs text-slate-200">{c.events.map((e) => coachEventLabels[e]).join(L("、"))}</p>
                  {/* 1回プランの受付状態（本体 #932）。受付停止中も一覧から外さず、表示だけで区別する。 */}
                  <p className={`text-xs font-bold ${c.on_duty ? "text-blue-400" : "text-red-400"}`}>{c.on_duty ? L("受付中") : L("受付停止中")}</p>
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
