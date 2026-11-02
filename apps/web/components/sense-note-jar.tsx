import { senseGrainColor, senseJarLevel, senseJarSize } from "@/lib/sense-notes";

// 感覚がたまっていくビン。1件書くごとに粒が1つ増え、6粒で満杯になって次のビンに移る。
// dropLatest を付けると、いちばん新しい粒が上から落ちてくる(「動きを減らす」設定の人には動かさない)。
// senseJarSize 粒でビンの中を埋める並び(下から2つずつ3段)。隣どうしが触れる大きさにして、
// ビンの中の余白が目立たないようにする。粒の数を変えたら、この並びも合わせ直す。
// 直径42の粒を3段積める高さ128の胴に収め、上段を切らない。
const grains = [
  { cx: 39, cy: 150, r: 21 },
  { cx: 81, cy: 150, r: 21 },
  { cx: 39, cy: 108, r: 21 },
  { cx: 81, cy: 108, r: 21 },
  { cx: 39, cy: 66, r: 21 },
  { cx: 81, cy: 66, r: 21 },
];

export function SenseNoteJar({ total, dropLatest = false, className = "" }: { total: number; dropLatest?: boolean; className?: string }) {
  const { filled, current } = senseJarLevel(total);
  // 満杯になった直後は、空のビンではなく満杯のビンを見せる。
  const shown = total > 0 && current === 0 ? senseJarSize : current;
  const jarIndex = total > 0 && current === 0 ? filled : filled + 1;
  const fillTop = shown === 0 ? 168 : shown === senseJarSize ? 44 : Math.max(44, Math.min(...grains.slice(0, shown).map((g) => g.cy - g.r)) + 6);
  return (
    <div className={`relative shrink-0 ${className}`}>
      <svg viewBox="0 0 120 184" className="h-full w-full" role="img" aria-label={`感覚のビン ${jarIndex}杯目、${shown}/${senseJarSize}粒`}>
        <defs>
          <linearGradient id="sense-jar-glass" x1="0" x2="1">
            <stop offset="0" style={{ stopColor: "color-mix(in oklab, var(--tt-white) 22%, transparent)" }} />
            <stop offset="0.5" style={{ stopColor: "color-mix(in oklab, var(--tt-white) 6%, transparent)" }} />
            <stop offset="1" style={{ stopColor: "color-mix(in oklab, var(--tt-white) 16%, transparent)" }} />
          </linearGradient>
          <linearGradient id="sense-jar-glow" x1="0" y1="0" x2="0" y2="1">
            <stop offset="0" style={{ stopColor: "color-mix(in oklab, var(--tt-blue-400) 0%, transparent)" }} />
            <stop offset="1" style={{ stopColor: "color-mix(in oklab, var(--tt-blue-400) 28%, transparent)" }} />
          </linearGradient>
          <clipPath id="sense-jar-inside">
            <path d="M22 44 Q14 50 14 64 V160 Q14 172 28 172 H92 Q106 172 106 160 V64 Q106 50 98 44 Z" />
          </clipPath>
        </defs>
        <rect x="34" y="16" width="52" height="14" rx="5" style={{ fill: "var(--tt-blue-400)" }} />
        <rect x="38" y="28" width="44" height="12" rx="3" style={{ fill: "color-mix(in oklab, var(--tt-white) 18%, transparent)" }} />
        <path
          d="M22 44 Q14 50 14 64 V160 Q14 172 28 172 H92 Q106 172 106 160 V64 Q106 50 98 44 Z"
          fill="url(#sense-jar-glass)"
          style={{ stroke: "color-mix(in oklab, var(--tt-white) 40%, transparent)" }}
          strokeWidth="2.5"
        />
        <g clipPath="url(#sense-jar-inside)">
          <rect x="0" y={fillTop} width="120" height={184 - fillTop} fill="url(#sense-jar-glow)" className="transition-all duration-700" />
          {Array.from({ length: shown }, (_, i) => {
            const { cx, cy, r } = grains[i % grains.length];
            const latest = dropLatest && i === shown - 1;
            return (
              <circle
                key={`${jarIndex}-${i}`}
                cx={cx}
                cy={cy}
                r={r}
                fill={senseGrainColor(i + filled * 7)}
                stroke="rgb(15 23 42 / 0.25)"
                strokeWidth="1"
                className={latest ? "sense-grain-drop" : undefined}
              />
            );
          })}
        </g>
        <path d="M24 60 Q22 108 24 150" style={{ stroke: "color-mix(in oklab, var(--tt-white) 30%, transparent)" }} strokeWidth="3" strokeLinecap="round" fill="none" />
      </svg>
      {filled > 0 ? (
        <span className="absolute -right-1 -top-1 rounded-full border border-amber-200/60 bg-amber-400/90 px-2 py-0.5 text-xs font-bold text-slate-900 shadow">
          {jarIndex}杯目
        </span>
      ) : null}
    </div>
  );
}
