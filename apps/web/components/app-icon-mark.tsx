import type { SVGProps } from "react";

// アプリアイコンの色。背景は上の明るい空色から下の青へのグラデーションに、右上から白い光を重ねる。
// レーンは白で、外側ほど濃く、内側ほど透ける。
export const appIconColors = {
  from: "#7fdcff",
  to: "#2d7dff",
  lane: "#ffffff",
} as const;

// 背景を CSS で描くときの値(AppIconMark と lib/app-icon.tsx で共通)。
export const appIconBackground = [
  "radial-gradient(circle at 80% 5%, rgba(255,255,255,0.4), rgba(255,255,255,0) 60%)",
  `linear-gradient(150deg, ${appIconColors.from}, ${appIconColors.to})`,
].join(", ");

// 512×512 のアイコン全体を基準にしたトラックの3レーン(外側→内側)。
// maskable でも欠けないよう、中央の安全領域(直径 80% の円)に収めてある。
export const appIconLanes = [
  { d: "M380 140H260A130 130 0 0 0 130 270V380", opacity: 1 },
  { d: "M380 190H260A80 80 0 0 0 180 270V380", opacity: 0.75 },
  { d: "M380 240H260A30 30 0 0 0 230 270V380", opacity: 0.5 },
] as const;

export const appIconLaneWidth = 30;

// アプリアイコン(lib/app-icon.tsx)と同じ、トラックの3レーンのマーク。
// アイコン全体と同じ大きさで重ねて使う。HTML の中でも、ほかの SVG の中(入れ子の <svg>)でもそのまま使える。
export function TrackLanesMark(props: SVGProps<SVGSVGElement>) {
  return (
    <svg viewBox="0 0 512 512" aria-hidden="true" {...props}>
      <g fill="none" stroke={appIconColors.lane} strokeWidth={appIconLaneWidth} strokeLinecap="round">
        {appIconLanes.map((lane) => (
          <path key={lane.d} d={lane.d} strokeOpacity={lane.opacity} />
        ))}
      </g>
    </svg>
  );
}

// ホーム画面に並ぶときと同じ見た目のアプリアイコン。
export function AppIconMark({ className = "" }: { className?: string }) {
  return (
    <span
      className={`flex shrink-0 overflow-hidden rounded-xl ring-1 ring-white/15 ${className}`}
      style={{ backgroundImage: appIconBackground }}
      aria-hidden
    >
      <TrackLanesMark className="size-full" />
    </span>
  );
}
