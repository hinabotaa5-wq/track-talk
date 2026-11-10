"use client";
import { L } from "@/lib/i18n";

import { PageLoading } from "@/components/page-loading";
import { CoachAvatar } from "@/components/coach-photo";
import { CoachEventChips, CoachEventEmpty, filterCoachesByEvent, type CoachEventFilter } from "@/components/coach-event-filter";
import { apiFetch } from "@/lib/api";
import { coachEventLabels } from "@/lib/coach-recruit";
import { supportPlanListEnabled } from "@/lib/features";
import { formatYen } from "@/lib/format";
import { priceHintLabel } from "@/lib/support-consult";
import type { SupportPlanCoach } from "@/lib/types";
import Link from "next/link";
import { useEffect, useState } from "react";

// サポートプランの入口の一覧（本体 #486）。動画添削の指名一覧とは別のリスト。
// 並びは API の公平なローテーションのまま。★・件数・売上・残り枠などの人気の指標は出さない。
type Filter = "all" | "competition_support" | "in_person";
const filters: { id: Filter; label: string }[] = [
  { id: "all", get label() { return L("すべて"); } },
  { id: "competition_support", get label() { return L("大会まで"); } },
  { id: "in_person", get label() { return L("対面"); } },
];

function CoachCard({ c, filter }: { c: SupportPlanCoach; filter: Filter }) {
  const showCompetition = c.competition_support && filter !== "in_person";
  const showInPerson = c.in_person && filter !== "competition_support";
  return (
    <li id={`support-coach-${c.id}`} className="min-w-0">
      <Link href={`/coaches/${encodeURIComponent(c.id)}`} className="glass flex h-full gap-3 rounded-xl border border-white/20 bg-white/10 p-3 text-sm transition-colors hover:border-blue-400/50 hover:bg-white/15">
        <CoachAvatar url={c.photo_url} name={c.nickname} size={56} />
        <div className="min-w-0 flex-1 space-y-1.5 break-words">
          <p className="font-bold text-white">{c.nickname ?? L("名前なし")}</p>
          <p className="text-xs text-slate-200">{c.events.map((e) => coachEventLabels[e]).join(L("、"))}</p>
          <p className="flex flex-wrap gap-1 text-xs">
            {c.kinds.map((k) => <span key={k} className="rounded-full border border-blue-400/40 px-2 py-0.5 text-blue-200">{k === "competition_support" ? L("大会まで") : L("対面")}</span>)}
          </p>
          {showCompetition ? (
            <div className="text-xs text-slate-200">
              <span className="font-semibold text-slate-100">{L("大会まで")}</span>{" "}
              {c.competition_support!.reception.status === "open" ? L("受付中") : L("受付停止中")}
              {priceHintLabel(c.competition_support!.price_min_yen, c.competition_support!.price_max_yen) ? L(" · 目安 {v0}", { v0: priceHintLabel(c.competition_support!.price_min_yen, c.competition_support!.price_max_yen) }) : ""}
              {c.competition_support!.reception.status === "open" ? L(" · 無料相談できます") : ""}
            </div>
          ) : null}
          {showInPerson ? (
            <div className="text-xs text-slate-200">
              <span className="font-semibold text-slate-100">{L("対面")}</span>{" "}
              {c.in_person!.reception.status === "open" ? L("受付中") : L("受付停止中")}
              {c.in_person!.areas.length ? ` · ${c.in_person!.areas.join(L("、"))}` : ""} · {formatYen(c.in_person!.min_price_yen)}
              {c.in_person!.plans.some((p) => p.price_yen !== c.in_person!.min_price_yen) ? L("〜") : ""}
            </div>
          ) : null}
        </div>
      </Link>
    </li>
  );
}

export function SupportPlanList() {
  const [coaches, setCoaches] = useState<SupportPlanCoach[] | null>(null);
  const [error, setError] = useState("");
  const [filter, setFilter] = useState<Filter>("all");
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
  // サービスの絞り込みと種目の絞り込みは AND で効く。並びは API の順のまま。
  const shown = filterCoachesByEvent((coaches ?? []).filter((c) => filter === "all" || c.kinds.includes(filter)), event);
  return (
    <div className="space-y-3">
      <div role="radiogroup" aria-label={L("絞り込み")} className="grid grid-cols-3 gap-1 rounded-xl border border-white/20 bg-white/5 p-1 text-sm">
        {filters.map((f) => (
          <button key={f.id} type="button" role="radio" aria-checked={filter === f.id} onClick={() => setFilter(f.id)} className={`rounded-lg px-2 py-1.5 ${filter === f.id ? "bg-blue-600 text-white" : "text-slate-200"}`}>{f.label}</button>
        ))}
      </div>
      <CoachEventChips value={event} onChange={setEvent} />
      {error ? <p role="alert" className="text-sm text-red-400">{error}</p> : null}
      {!coaches && !error ? <PageLoading label={L("読み込み中...")} /> : null}
      {coaches && shown.length === 0 && event === "all" ? <p className="text-sm text-slate-200">{L("まだいません")}</p> : null}
      {coaches && shown.length === 0 && event !== "all" ? <CoachEventEmpty onReset={() => setEvent("all")} /> : null}
      <ul className="space-y-3">{shown.map((c) => <CoachCard key={c.id} c={c} filter={filter} />)}</ul>
    </div>
  );
}
