"use client";

import { InjuryBodyFigure } from "@/components/injury-body-figure";
import { injuryRegionSpots, type InjuryFigureSide, type InjuryRegion } from "@/lib/injury";

const sides: { side: InjuryFigureSide; label: string }[] = [
  { side: "front", label: "まえ" },
  { side: "back", label: "うしろ" },
];

// まえ・うしろの人型に、まとまりごとの人数の丸を置く。丸を押すとそのまとまりを選ぶ。
// キーボードと読み上げでは下の絞り込みボタンで選ぶので、図そのものは飾りとして隠す。
export function InjuryBodyMap({
  members,
  selected,
  onSelect,
}: {
  members: Record<InjuryRegion, number>;
  selected: InjuryRegion;
  onSelect: (region: InjuryRegion) => void;
}) {
  return (
    <div className="flex justify-center gap-6">
      {sides.map(({ side, label }) => (
        <figure key={side} className="flex flex-col items-center gap-0.5">
          <InjuryBodyFigure side={side} className="h-56 w-28">
            {injuryRegionSpots
              .filter((spot) => spot.side === side)
              .map((spot) => {
                const active = spot.region === selected;
                return (
                  <g key={spot.region} className="cursor-pointer" onClick={() => onSelect(spot.region)}>
                    <circle
                      cx={spot.x}
                      cy={spot.y}
                      r={9}
                      className={`animate-pulse motion-reduce:animate-none ${active ? "fill-blue-400/35" : "fill-orange-400/30"}`}
                    />
                    <circle cx={spot.x} cy={spot.y} r={5.6} strokeWidth={1.2} className={`stroke-[#16243a] ${active ? "fill-blue-400" : "fill-orange-400"}`} />
                    <text x={spot.x} y={spot.y} textAnchor="middle" dominantBaseline="central" fontSize={6} fontWeight={700} className="pointer-events-none fill-white">
                      {members[spot.region]}
                    </text>
                  </g>
                );
              })}
          </InjuryBodyFigure>
          <figcaption className="text-xs text-slate-400">{label}</figcaption>
        </figure>
      ))}
    </div>
  );
}
