import { ImageResponse } from "next/og";
import { appIconColors, appIconLanes } from "@/components/app-icon-mark";

// ヘッダーと同じB案。マークはmaskableアイコンの安全領域内に収める。
export function renderAppIcon(size: number) {
  return new ImageResponse(
    <div style={{ width: "100%", height: "100%", display: "flex", backgroundColor: "#ffffff" }}>
      <svg width={size} height={size} viewBox="0 0 512 512">
        <defs>
          <linearGradient id="icon-lane-blue" x1="0" y1="0" x2="1" y2="1">
            <stop stopColor={appIconColors.from} />
            <stop offset="1" stopColor={appIconColors.to} />
          </linearGradient>
        </defs>
        <g fill="none" stroke="url(#icon-lane-blue)" strokeWidth="30" strokeLinecap="round">
          {appIconLanes.map((d) => <path key={d} d={d} />)}
        </g>
      </svg>
    </div>,
    { width: size, height: size },
  );
}
