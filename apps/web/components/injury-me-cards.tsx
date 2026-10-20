"use client";

import Link from "next/link";
import { HeartPulse } from "lucide-react";
import { useEffect, useState } from "react";
import { Button } from "./ui/button";
import { InjuryGoalCelebration } from "./injury-goal-celebration";
import { InjuryStageChange } from "./injury-stage-change";
import { apiFetch } from "@/lib/api";
import {
  injuryElapsedText,
  injuryGroupPath,
  isCurrentInjuryStage,
  sortInjuryGroups,
  type InjuryGroupMember,
  type InjuryGroupSummary,
} from "@/lib/injury";

type Joined = { part: string; label: string; me: InjuryGroupMember };

// マイページの一番上に出す、いまのケガと段階。入っているグループごとに1枚。どこにも入っていなければ何も出さない。
// 大会に復帰したケガはもう「いまのケガ」ではないので出さない。段階の記録はグループにそのまま残る。
export function InjuryMeCards({ token }: { token: string | null }) {
  const [joined, setJoined] = useState<Joined[]>([]);
  const [changing, setChanging] = useState<string | null>(null);
  const [celebrating, setCelebrating] = useState<string | null>(null);

  useEffect(() => {
    if (!token) return;
    const controller = new AbortController();
    apiFetch<{ groups: InjuryGroupSummary[] }>("/v1/injury/groups", { token, signal: controller.signal })
      .then((d) =>
        setJoined(
          sortInjuryGroups(d.groups ?? []).flatMap((g) => (g.me ? [{ part: g.body_part, label: g.body_part_label, me: g.me }] : [])),
        ),
      )
      .catch(() => {
        // 読めなくても、マイページのほかの部分はそのまま使える。
      });
    return () => controller.abort();
  }, [token]);

  const current = joined.filter((item) => isCurrentInjuryStage(item.me.stage));
  if (current.length === 0 && celebrating === null) return null;
  const now = new Date();

  return (
    <section aria-label="いまのケガ" className="space-y-3 md:col-span-2">
      {/* お祝いは「大会に復帰」に変えた直後だけ。そのケガのカードは消えるので、カードの代わりにここへ出す。 */}
      {celebrating !== null ? <InjuryGoalCelebration onClose={() => setCelebrating(null)} /> : null}
      {current.map((item) => {
        const label = item.me.diagnosis ? `${item.label}(${item.me.diagnosis})` : item.label;
        const elapsed = injuryElapsedText(item.me.injured_on, now);
        return (
          <div key={item.part} className="space-y-3">
            <div className="flex flex-wrap items-center gap-3 rounded-2xl border border-white/20 glass bg-white/10 p-4">
              <HeartPulse className="size-6 shrink-0 text-rose-300" aria-hidden />
              <Link href={injuryGroupPath(item.part)} className="min-w-0 flex-1 hover:underline">
                <p className="text-xs text-slate-300">いまのケガ</p>
                <p className="font-bold">
                  {label}・{item.me.stage_label}
                  {elapsed ? <span className="font-normal text-slate-200">({elapsed})</span> : null}
                </p>
              </Link>
              <Button
                variant="secondary"
                className="shrink-0"
                aria-expanded={changing === item.part}
                onClick={() => setChanging((current) => (current === item.part ? null : item.part))}
              >
                段階を変える
              </Button>
            </div>
            {changing === item.part ? (
              <InjuryStageChange
                part={item.part}
                token={token}
                member={item.me}
                onCancel={() => setChanging(null)}
                onChanged={(me, celebrate) => {
                  setJoined((current) => current.map((j) => (j.part === item.part ? { ...j, me } : j)));
                  setChanging(null);
                  if (celebrate) setCelebrating(item.part);
                }}
              />
            ) : null}
          </div>
        );
      })}
    </section>
  );
}
