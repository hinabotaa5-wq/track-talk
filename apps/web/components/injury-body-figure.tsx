import type { ReactNode } from "react";
import { injuryPartFigure, type InjuryFigureSide } from "@/lib/injury";

// 丸と太い線だけで描いた人型(viewBox 0 0 60 120)。うしろ向きは足先の向きと背骨の線で見分ける。
// children はこの座標のまま、人型の上に重ねて描く。
export function InjuryBodyFigure({
  side,
  className,
  children,
}: {
  side: InjuryFigureSide;
  className?: string;
  children?: ReactNode;
}) {
  const back = side === "back";
  return (
    <svg viewBox="0 0 60 120" className={className} aria-hidden overflow="visible">
      <g className="stroke-slate-500" strokeLinecap="round" fill="none">
        <path d="M19 21 L14 40 L11 58" strokeWidth={5.5} />
        <path d="M41 21 L46 40 L49 58" strokeWidth={5.5} />
        <path d="M25.5 58 L25 85 L25 112" strokeWidth={8} />
        <path d="M34.5 58 L35 85 L35 112" strokeWidth={8} />
        <path d={`M25 114 L${back ? 27 : 20} 116`} strokeWidth={5} />
        <path d={`M35 114 L${back ? 33 : 40} 116`} strokeWidth={5} />
      </g>
      <path d="M18 19 Q30 15 42 19 L41 46 Q41 60 30 61 Q19 60 19 46 Z" className="fill-slate-500" />
      <circle cx={30} cy={8.5} r={7} className="fill-slate-500" />
      {back ? <path d="M30 22 L30 50" className="stroke-slate-400" strokeWidth={1} strokeLinecap="round" /> : null}
      {children}
    </svg>
  );
}

// ケガ1つぶんの小さな人型。痛いところを光らせる。場所の決まらない「その他」は「?」を出す。
export function InjuryPartIcon({ part, className }: { part: string; className?: string }) {
  const figure = injuryPartFigure(part);
  return (
    <InjuryBodyFigure side={figure.side} className={className}>
      {figure.spots.length === 0 ? (
        <>
          <circle cx={30} cy={38} r={10} className="fill-orange-400/30" />
          <text x={30} y={39} textAnchor="middle" dominantBaseline="central" fontSize={13} fontWeight={800} className="fill-orange-300">
            ?
          </text>
        </>
      ) : (
        figure.spots.map(([x, y]) => (
          <g key={`${x}-${y}`}>
            <circle cx={x} cy={y} r={10} className="fill-orange-400/30" />
            <circle cx={x} cy={y} r={4.6} className="fill-orange-400" />
          </g>
        ))
      )}
    </InjuryBodyFigure>
  );
}
