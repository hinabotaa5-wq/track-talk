import type { SVGProps } from "react";

// アプリアイコンの色。背景は明るい青のグラデーション(左上→右下)。
export const appIconColors = {
  from: "#3b82f6",
  to: "#22d3ee",
  outer: "#ffffff",
  middle: "#1e3a8a",
  inner: "#ffffff",
} as const;

// アプリアイコン(lib/app-icon.tsx)と同じ、トラックの3レーンのマーク。
// HTML の中でも、ほかの SVG の中(入れ子の <svg>)でもそのまま使える。
export function TrackLanesMark(props: SVGProps<SVGSVGElement>) {
  return (
    <svg viewBox="4 6 42 38" aria-hidden="true" {...props}>
      <g fill="none" strokeWidth="2.6" strokeLinecap="round" strokeLinejoin="round">
        <path d="M42 10H28A20 20 0 0 0 8 30v10" stroke={appIconColors.outer} />
        <path d="M42 16.5H28A13.5 13.5 0 0 0 14.5 30v10" stroke={appIconColors.middle} />
        <path d="M42 23H28A7 7 0 0 0 21 30v10" stroke={appIconColors.inner} />
      </g>
    </svg>
  );
}

// ホーム画面に並ぶときと同じ見た目のアプリアイコン。
export function AppIconMark({ className = "" }: { className?: string }) {
  return (
    <span
      className={`flex shrink-0 items-center justify-center rounded-xl ring-1 ring-white/15 ${className}`}
      style={{ backgroundImage: `linear-gradient(135deg, ${appIconColors.from}, ${appIconColors.to})` }}
      aria-hidden
    >
      <TrackLanesMark className="h-auto w-[56%]" />
    </span>
  );
}
