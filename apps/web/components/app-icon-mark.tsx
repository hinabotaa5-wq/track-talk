import type { SVGProps } from "react";

export const appIconColors = {
  from: "#2ec5ff",
  to: "#0a5cff",
} as const;

export const appIconLanes = [
  "M380 140H260A130 130 0 0 0 130 270V380",
  "M380 190H260A80 80 0 0 0 180 270V380",
  "M380 240H260A30 30 0 0 0 230 270V380",
] as const;

// B案：白地に、水色から青へ変わるトラックの3レーン。
export function TrackLanesMark(props: SVGProps<SVGSVGElement>) {
  return (
    <svg viewBox="0 0 512 512" aria-hidden="true" {...props}>
      <defs>
        <linearGradient id="tracktalk-lane-blue" x1="0" y1="0" x2="1" y2="1">
          <stop stopColor={appIconColors.from} />
          <stop offset="1" stopColor={appIconColors.to} />
        </linearGradient>
      </defs>
      <g fill="none" stroke="url(#tracktalk-lane-blue)" strokeWidth="30" strokeLinecap="round">
        {appIconLanes.map((d) => <path key={d} d={d} />)}
      </g>
    </svg>
  );
}

export function AppIconMark({ className = "" }: { className?: string }) {
  return (
    <span className={`flex shrink-0 items-center justify-center overflow-hidden rounded-xl bg-white ${className}`} aria-hidden>
      <TrackLanesMark className="size-full" />
    </span>
  );
}

// アプリ内もホーム画面と同じマークを使う。ヘッダーでは白地を敷かず、濃紺の上にレーンだけを乗せる。
// 白地の余白がなくなる分、viewBox をレーンの範囲に切り詰めて大きさを保つ。
// タイトルは暗いヘッダーでも読める青にする。
export function AppLogo() {
  return (
    <span className="inline-flex shrink-0 items-center gap-2.5 whitespace-nowrap">
      <TrackLanesMark viewBox="112 122 286 286" className="size-8 shrink-0" />
      <span
        className="bg-clip-text text-xl font-black leading-none tracking-tight text-transparent"
        style={{
          backgroundImage: `linear-gradient(135deg, ${appIconColors.from}, color-mix(in srgb, ${appIconColors.to} 60%, white))`,
        }}
      >
        TrackTalk
      </span>
    </span>
  );
}
