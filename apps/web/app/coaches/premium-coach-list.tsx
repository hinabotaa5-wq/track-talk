"use client";
import { L } from "@/lib/i18n";

import { PageLoading } from "@/components/page-loading";
import { CoachAvatar } from "@/components/coach-photo";
import { apiFetch } from "@/lib/api";
import { formatYen } from "@/lib/format";
import { coachEventLabels } from "@/lib/coach-recruit";
import { coachDisplayText, countryFlagEmoji } from "@/lib/coach-display";
import { maxPremiumVideos, premiumRequestHref, premiumSlotLabels, premiumSlots } from "@/lib/premium";
import type { PublicCoach } from "@/lib/types";
import Link from "next/link";
import { useEffect, useState } from "react";

// コーチ一覧のプレミアムタブ（本体 #1053）。プレミアム価格があるコーチだけを、その価格と一緒に出す。
// 価格は API の有効価格（premium_price_yen）をそのまま出す。受付停止中のコーチは一覧に残し、申込みの導線だけを止める。
export function PremiumCoachList({ onSettled }: { onSettled?: () => void }) {
  const [coaches, setCoaches] = useState<PublicCoach[] | null>(null);
  const [error, setError] = useState("");

  useEffect(() => {
    let cancelled = false;
    apiFetch<{ coaches: PublicCoach[] }>("/v1/coaches")
      .then((d) => {
        if (cancelled) return;
        setCoaches(d.coaches.filter((c) => typeof c.premium_price_yen === "number"));
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
  if (!coaches) return <PageLoading screen label={L("読み込み中...")} />;

  return (
    <div className="space-y-3">
      <p className="text-sm text-slate-200">
        {L("決まった5枠（{slots}）に、合わせて{max}本まで動画を送れます。コーチが全部を見て、総括で返します。", {
          slots: premiumSlots.map((slot) => premiumSlotLabels[slot]).join(L("、")),
          max: maxPremiumVideos,
        })}
      </p>
      {coaches.length === 0 ? <p className="text-sm text-slate-200">{L("まだいません")}</p> : (
        <ul data-testid="premium-coach-list" className="grid grid-cols-2 gap-3">
          {coaches.map((c) => {
            const displayName = coachDisplayText(c.nickname, c.nickname_en);
            const flag = countryFlagEmoji(c.country_code);
            const body = (
              <>
                <CoachAvatar url={c.photo_url} name={displayName} size={80} />
                <div className="w-full min-w-0 space-y-1 break-words">
                  <p className="font-bold text-white">
                    {flag ? <span aria-hidden="true">{flag} </span> : null}
                    {displayName ?? L("名前なし")}
                  </p>
                  <p className="text-xs text-slate-200">{c.events.map((e) => coachEventLabels[e]).join(L("、"))}</p>
                  <p className={`text-xs font-bold ${c.on_duty ? "text-blue-400" : "text-red-400"}`}>{c.on_duty ? L("受付中") : L("受付停止中")}</p>
                  <p className="font-semibold text-blue-300">{formatYen(c.premium_price_yen ?? 0)}</p>
                </div>
              </>
            );
            return (
              <li key={c.id} className="min-w-0">
                {c.on_duty ? (
                  <Link href={premiumRequestHref(c.id)} className="app-card flex h-full flex-col items-center gap-3 border border-transparent p-4 text-center text-sm transition-all hover:border-blue-400/50 hover:shadow-md focus-visible:outline-2 focus-visible:outline-blue-400">
                    {body}
                  </Link>
                ) : (
                  <div aria-disabled="true" className="app-card flex h-full flex-col items-center gap-3 p-4 text-center text-sm opacity-60">{body}</div>
                )}
              </li>
            );
          })}
        </ul>
      )}
    </div>
  );
}
