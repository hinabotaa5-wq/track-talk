import type { SVGProps } from "react";

// アプリアイコン(lib/app-icon.tsx)と同じ、トラックの3レーンのマーク。
// HTML の中でも、ほかの SVG の中(入れ子の <svg>)でもそのまま使える。
export function TrackLanesMark(props: SVGProps<SVGSVGElement>) {
  return (
    <svg viewBox="4 6 42 38" aria-hidden="true" {...props}>
      <g fill="none" strokeWidth="2.6" strokeLinecap="round" strokeLinejoin="round">
        <path d="M42 10H28A20 20 0 0 0 8 30v10" stroke="#ffffff" />
        <path d="M42 16.5H28A13.5 13.5 0 0 0 14.5 30v10" stroke="#93c5fd" />
        <path d="M42 23H28A7 7 0 0 0 21 30v10" stroke="rgba(255,255,255,0.8)" />
      </g>
    </svg>
  );
}

// ホーム画面に並ぶときと同じ見た目のアプリアイコン。
export function AppIconMark({ className = "" }: { className?: string }) {
  return (
    <span
      className={`flex shrink-0 items-center justify-center rounded-xl bg-[#16243a] ring-1 ring-white/15 ${className}`}
      aria-hidden
    >
      <TrackLanesMark className="h-auto w-[56%]" />
    </span>
  );
}
