"use client";

import { useEffect, useState } from "react";
import { apiFetch } from "@/lib/api";
import { injuryStageStatText, injuryTypicalStages, type InjuryStats } from "@/lib/injury";

export function InjuryTypicalRecovery({ bodyPart, bodyPartLabel }: { bodyPart: string; bodyPartLabel: string }) {
  const [stats, setStats] = useState<InjuryStats | null>(null);

  useEffect(() => {
    const controller = new AbortController();
    apiFetch<InjuryStats>("/v1/injury/stats", { signal: controller.signal })
      .then(setStats)
      .catch(() => {
        // 目安は補足の情報なので、取得できなければ何も出さない。
      });
    return () => controller.abort();
  }, []);

  const stages = injuryTypicalStages(stats, bodyPart);
  if (!stats || stages.length === 0) return null;

  return (
    <section className="space-y-2 rounded-xl border border-white/20 glass bg-white/10 p-4">
      <h2 className="text-base font-semibold text-slate-100">{bodyPartLabel}の復帰の目安</h2>
      <dl className="space-y-1.5 text-sm">
        {stages.map((stage) => (
          <div key={stage.stage} className="flex justify-between gap-3">
            <dt className="font-medium text-slate-100">{stage.stage_label}</dt>
            <dd className="text-right text-slate-200">{injuryStageStatText(stage, stats.min_sample)}</dd>
          </div>
        ))}
      </dl>
      <p className="text-xs text-slate-300">
        みんなの経過報告の中央値で、回復を保証するものではありません。
      </p>
    </section>
  );
}
