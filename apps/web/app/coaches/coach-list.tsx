"use client";

import { CoachAvatar } from "@/components/coach-photo";
import { PublicPlanList } from "@/components/coaching-plans";
import { apiFetch } from "@/lib/api";
import { coachingServiceLabels, serviceReady } from "@/lib/coaching-plan";
import { coachEventLabels, coachTierLabels } from "@/lib/coach-recruit";
import type { CoachTier, PublicCoach } from "@/lib/types";
import { useEffect, useState } from "react";

const chip = (on: boolean) =>
  `rounded-full border px-3 py-1.5 text-sm transition active:scale-[0.98] ${
    on ? "border-blue-400 bg-blue-500/30 text-white" : "border-white/25 bg-white/10 text-slate-200"
  }`;

export function CoachList() {
  const [coaches, setCoaches] = useState<PublicCoach[] | null>(null);
  const [error, setError] = useState("");
  const [tier, setTier] = useState<CoachTier | "all">("all");

  useEffect(() => {
    let cancelled = false;
    apiFetch<{ coaches: PublicCoach[] }>("/v1/coaches")
      .then((d) => {
        if (!cancelled) setCoaches(d.coaches);
      })
      .catch(() => {
        if (!cancelled) setError("読み込みに失敗しました");
      });
    return () => {
      cancelled = true;
    };
  }, []);

  if (error) return <p className="text-sm text-red-400">{error}</p>;
  if (!coaches) return <p className="text-sm text-slate-200">読み込み中...</p>;

  const shown = tier === "all" ? coaches : coaches.filter((c) => c.tier === tier);

  return (
    <div className="space-y-4">
      <div className="flex flex-wrap gap-2">
        <button type="button" aria-pressed={tier === "all"} onClick={() => setTier("all")} className={chip(tier === "all")}>
          すべて
        </button>
        {(Object.keys(coachTierLabels) as CoachTier[]).map((t) => (
          <button key={t} type="button" aria-pressed={tier === t} onClick={() => setTier(t)} className={chip(tier === t)}>
            {coachTierLabels[t]}
          </button>
        ))}
      </div>
      {shown.length === 0 ? <p className="text-sm text-slate-200">まだいません</p> : null}
      <ul className="space-y-3">
        {shown.map((c) => (
          <li id={`coach-${c.id}`} key={c.id} className="space-y-2 rounded-xl border border-white/20 glass bg-white/10 p-3 text-sm">
            <div className="flex items-center gap-3">
              <CoachAvatar url={c.photo_url} name={c.nickname} />
              <div className="min-w-0 flex-1">
                <p className="font-bold text-white">{c.nickname ?? "名前なし"}</p>
                <p className="text-xs text-slate-300">{coachTierLabels[c.tier]}</p>
                <p className="text-xs text-slate-200">
                  {c.rating_count > 0 ? `★${c.rating_average.toFixed(1)}（${c.rating_count}件）` : "評価はまだありません"}
                </p>
              </div>
            </div>
            {(c.service_types ?? []).length > 0 ? <p className="text-xs text-slate-200">
              担当サービス：{c.service_types.map((s) => `${coachingServiceLabels[s]}${serviceReady(s) ? "" : "（準備中）"}`).join("・")}
            </p> : null}
            <dl className="space-y-1.5">
              <div>
                <dt className="text-xs text-slate-300">担当種目</dt>
                <dd className="text-slate-100">{c.events.map((e) => coachEventLabels[e]).join("、")}</dd>
              </div>
              <div>
                <dt className="text-xs text-slate-300">記録</dt>
                <dd className="whitespace-pre-wrap text-slate-100">{c.best_record}</dd>
              </div>
              <div>
                <dt className="text-xs text-slate-300">成績</dt>
                <dd className="whitespace-pre-wrap text-slate-100">{c.achievements}</dd>
              </div>
              <div>
                <dt className="text-xs text-slate-300">競技年数</dt>
                <dd className="text-slate-100">{c.years}年</dd>
              </div>
            </dl>
            <PublicPlanList plans={c.plans ?? []} />
          </li>
        ))}
      </ul>
    </div>
  );
}
