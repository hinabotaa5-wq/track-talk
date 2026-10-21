import { senseGrainColor, senseJarLevel, senseJarSize } from "@/lib/sense-notes";

// 感覚がたまっていくビン。1件書くごとに粒が1つ増え、6粒で満杯になって次のビンに移る。
// dropLatest を付けると、いちばん新しい粒が上から落ちてくる(「動きを減らす」設定の人には動かさない)。
// 粒の並びは、senseJarSize 粒でビンの中がちょうど埋まる形(3列×2段)。粒の数を変えたら合わせ直す。
const columns = 3;
const grainRadius = 14.5;
const grainStep = 29;
const rowStep = 40;

function grainPosition(index: number) {
  const row = Math.floor(index / columns);
  const col = index % columns;
  return { cx: 60 + (col - 1) * grainStep, cy: 135 - row * rowStep };
}

export function SenseNoteJar({ total, dropLatest = false, className = "" }: { total: number; dropLatest?: boolean; className?: string }) {
  const { filled, current } = senseJarLevel(total);
  // 満杯になった直後は、空のビンではなく満杯のビンを見せる。
  const shown = total > 0 && current === 0 ? senseJarSize : current;
  const jarIndex = total > 0 && current === 0 ? filled : filled + 1;
  const fillTop = shown === 0 ? 146 : 135 - (Math.ceil(shown / columns) - 1) * rowStep - grainRadius + 6;
  return (
    <div className={`relative shrink-0 ${className}`}>
      <svg viewBox="0 0 120 160" className="h-full w-full" role="img" aria-label={`感覚のビン ${jarIndex}杯目、${shown}/${senseJarSize}粒`}>
        <defs>
          <linearGradient id="sense-jar-glass" x1="0" x2="1">
            <stop offset="0" stopColor="rgb(255 255 255 / 0.22)" />
            <stop offset="0.5" stopColor="rgb(255 255 255 / 0.06)" />
            <stop offset="1" stopColor="rgb(255 255 255 / 0.16)" />
          </linearGradient>
          <linearGradient id="sense-jar-glow" x1="0" y1="0" x2="0" y2="1">
            <stop offset="0" stopColor="rgb(125 211 252 / 0)" />
            <stop offset="1" stopColor="rgb(125 211 252 / 0.28)" />
          </linearGradient>
          <clipPath id="sense-jar-inside">
            <path d="M22 44 Q14 50 14 64 V138 Q14 150 28 150 H92 Q106 150 106 138 V64 Q106 50 98 44 Z" />
          </clipPath>
        </defs>
        <rect x="34" y="16" width="52" height="14" rx="5" fill="rgb(253 230 138 / 0.85)" />
        <rect x="38" y="28" width="44" height="12" rx="3" fill="rgb(255 255 255 / 0.18)" />
        <path
          d="M22 44 Q14 50 14 64 V138 Q14 150 28 150 H92 Q106 150 106 138 V64 Q106 50 98 44 Z"
          fill="url(#sense-jar-glass)"
          stroke="rgb(255 255 255 / 0.55)"
          strokeWidth="2.5"
        />
        <g clipPath="url(#sense-jar-inside)">
          <rect x="0" y={fillTop} width="120" height={160 - fillTop} fill="url(#sense-jar-glow)" className="transition-all duration-700" />
          {Array.from({ length: shown }, (_, i) => {
            const { cx, cy } = grainPosition(i);
            const latest = dropLatest && i === shown - 1;
            return (
              <circle
                key={`${jarIndex}-${i}`}
                cx={cx}
                cy={cy}
                r={grainRadius}
                fill={senseGrainColor(i + filled * 7)}
                stroke="rgb(15 23 42 / 0.25)"
                strokeWidth="1"
                className={latest ? "sense-grain-drop" : undefined}
              />
            );
          })}
        </g>
        <path d="M24 60 Q22 90 24 128" stroke="rgb(255 255 255 / 0.45)" strokeWidth="3" strokeLinecap="round" fill="none" />
      </svg>
      {filled > 0 ? (
        <span className="absolute -right-1 -top-1 rounded-full border border-amber-200/60 bg-amber-400/90 px-2 py-0.5 text-xs font-bold text-slate-900 shadow">
          {jarIndex}杯目
        </span>
      ) : null}
    </div>
  );
}
