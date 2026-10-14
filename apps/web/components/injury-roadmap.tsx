"use client";

import { ArrowDown, Bandage, Flag, Footprints, HeartPulse, PersonStanding, type LucideIcon } from "lucide-react";
import { injuryFaceHue, injuryFaceInitial, injuryFaceOverflow, type InjuryGroupStage, type InjuryStage } from "@/lib/injury";

const stageStyle: Record<InjuryStage, { icon: LucideIcon; tone: string }> = {
  resting: { icon: Bandage, tone: "bg-red-950/60 text-red-300" },
  rehab: { icon: HeartPulse, tone: "bg-amber-950/60 text-amber-300" },
  jogging: { icon: PersonStanding, tone: "bg-blue-950/70 text-blue-300" },
  practice: { icon: Footprints, tone: "bg-emerald-950/60 text-emerald-300" },
  race: { icon: Flag, tone: "bg-emerald-950/60 text-emerald-300" },
};

function Faces({ stage }: { stage: InjuryGroupStage }) {
  const extra = injuryFaceOverflow(stage.members, stage.faces.length);
  if (stage.members === 0) return null;
  return (
    <div className="mt-2 flex items-center" aria-hidden="true">
      {stage.faces.map((name, index) => (
        <span
          key={`${name}-${index}`}
          className="-ml-1.5 flex size-6 items-center justify-center rounded-full border-2 border-slate-900 text-[11px] font-bold text-slate-900 first:ml-0"
          style={{ backgroundColor: `hsl(${injuryFaceHue(name)} 70% 75%)` }}
        >
          {injuryFaceInitial(name)}
        </span>
      ))}
      {extra > 0 ? <span className="ml-1.5 text-xs text-slate-300">+{extra}</span> : null}
    </div>
  );
}

// 怪我直後(上)からゴール(下)まで、矢印でつないだロードマップ。段階をタップするとその段階の投稿が読める。
export function InjuryRoadmap({
  stages,
  mine,
  selected,
  onSelect,
}: {
  stages: InjuryGroupStage[];
  mine: InjuryStage;
  selected: InjuryStage;
  onSelect: (stage: InjuryStage) => void;
}) {
  return (
    <ol className="space-y-0" aria-label="復帰までのロードマップ">
      <li className="mb-1 text-xs text-slate-300">スタート: ケガ直後</li>
      {stages.map((stage, index) => {
        const { icon: Icon, tone } = stageStyle[stage.stage];
        const isMine = stage.stage === mine;
        const isSelected = stage.stage === selected;
        return (
          <li key={stage.stage}>
            <button
              type="button"
              onClick={() => onSelect(stage.stage)}
              aria-pressed={isSelected}
              className="flex w-full items-center gap-3 text-left"
            >
              <span className={`flex size-10 shrink-0 items-center justify-center rounded-full ${tone}`}>
                <Icon className="size-5" aria-hidden="true" />
              </span>
              <span
                className={`flex-1 rounded-xl border px-3 py-2 transition active:scale-[0.99] ${
                  isSelected ? "border-2 border-blue-400 bg-blue-950/50" : "border-white/20 glass bg-white/8 hover:bg-white/12"
                }`}
              >
                <span className="block text-sm font-bold">
                  {stage.stage_label}
                  <span className="ml-1.5 font-normal text-slate-300">· {stage.members}人</span>
                </span>
                {isMine ? <span className="block text-xs font-semibold text-blue-200">あなたはここ</span> : null}
                {stage.stage === "race" && !isMine ? <span className="block text-xs text-slate-300">ゴール</span> : null}
                <Faces stage={stage} />
              </span>
            </button>
            {index < stages.length - 1 ? (
              <ArrowDown className="my-0.5 ml-[11px] size-[18px] text-slate-400" aria-hidden="true" />
            ) : null}
          </li>
        );
      })}
    </ol>
  );
}
