"use client";
import { L } from "@/lib/i18n";

import { PageLoading } from "@/components/page-loading";
import { CoachAvatar } from "@/components/coach-photo";
import { CoachEventChips, CoachEventEmpty, filterCoachesByEvent, type CoachEventFilter } from "@/components/coach-event-filter";
import { apiFetch } from "@/lib/api";
import { coachEventLabels } from "@/lib/coach-recruit";
import { supportPlanListEnabled } from "@/lib/features";
import type { SupportPlanCoach } from "@/lib/types";
import { priceHintLabel } from "@/lib/support-consult";
import Link from "next/link";
import { useEffect, useState } from "react";

// サポートプランの入口の一覧（本体 #486）。動画添削の指名一覧とは別のリスト。
// 種別の絞り込み（すべて／大会まで／対面）は置かない。初期表示は対象コーチ全員で、残る絞り込みは種目チップのみ
// （両一覧共通の既存部品）。対面は表示しない（コード・データは残し、API 側で無効のまま）。
// 並びは API の公平なローテーションのまま。★・件数・売上・残り枠などの人気の指標は出さない。
function CoachCard({ c }: { c: SupportPlanCoach }) {
  const cs = c.competition_support;
  if (!cs) return null;
  return (
    <li id={`support-coach-${c.id}`} className="min-w-0">
      <Link href={`/coaches/${encodeURIComponent(c.id)}`} className="glass flex h-full gap-3 rounded-xl border border-white/20 bg-white/10 p-3 text-sm transition-colors hover:border-blue-400/50 hover:bg-white/15">
        <CoachAvatar url={c.photo_url} name={c.nickname} size={56} />
        <div className="min-w-0 flex-1 space-y-1.5 break-words">
          <p className="font-bold text-white">{c.nickname ?? L("名前なし")}</p>
          <p className="text-xs text-slate-200">{c.events.map((e) => coachEventLabels[e]).join(L("、"))}</p>
          <div className="text-xs text-slate-200">
            <span className="font-semibold text-slate-100">{L("1ヶ月プラン")}</span>{" "}
            {cs.reception.status === "open" ? L("受付中") : L("受付停止中")}
            {priceHintLabel(cs.price_min_yen, cs.price_max_yen) ? L(" · 目安 {v0}", { v0: priceHintLabel(cs.price_min_yen, cs.price_max_yen) }) : ""}
            {cs.reception.status === "open" ? L(" · 無料相談できます") : ""}
          </div>
        </div>
      </Link>
    </li>
  );
}

export function SupportPlanList() {
  const [coaches, setCoaches] = useState<SupportPlanCoach[] | null>(null);
  const [error, setError] = useState("");
  const [event, setEvent] = useState<CoachEventFilter>("all");

  useEffect(() => {
    if (!supportPlanListEnabled) return;
    let cancelled = false;
    apiFetch<{ coaches: SupportPlanCoach[] }>("/v1/coaches/support-plan")
      .then((d) => { if (!cancelled) setCoaches(d.coaches); })
      .catch(() => { if (!cancelled) setError(L("読み込みに失敗しました")); });
    return () => { cancelled = true; };
  }, []);

  if (!supportPlanListEnabled) {
    return <p className="rounded-xl border border-blue-400/40 bg-blue-500/10 p-4 text-center font-semibold text-slate-100">{L("サポートプランは12月開始予定です")}</p>;
  }
  // 種目の絞り込みだけが効く。並びは API の順のまま。
  const shown = filterCoachesByEvent(coaches ?? [], event);
  return (
    <div className="space-y-3">
      <CoachEventChips value={event} onChange={setEvent} />
      {error ? <p role="alert" className="text-sm text-red-400">{error}</p> : null}
      {!coaches && !error ? <PageLoading label={L("読み込み中...")} /> : null}
      {coaches && shown.length === 0 && event === "all" ? <p className="text-sm text-slate-200">{L("まだいません")}</p> : null}
      {coaches && shown.length === 0 && event !== "all" ? <CoachEventEmpty onReset={() => setEvent("all")} /> : null}
      <ul className="space-y-3">{shown.map((c) => <CoachCard key={c.id} c={c} />)}</ul>
    </div>
  );
}
