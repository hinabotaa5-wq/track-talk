import { ImageResponse } from "next/og";
import {
  appIconBackground,
  appIconColors,
  appIconLanes,
  appIconLaneWidth,
} from "@/components/app-icon-mark";

// ヘッダーのロゴ(トラックの3レーン)をアプリアイコンにする。
// 明るい空色→青のグラデーションに右上から光を当て、白いレーンを外側ほど濃く重ねて奥行きを出す。
// 色と形は components/app-icon-mark.tsx と揃える。
export function renderAppIcon(size: number) {
  return new ImageResponse(
    (
      <div
        style={{
          width: "100%",
          height: "100%",
          display: "flex",
          backgroundImage: appIconBackground,
        }}
      >
        <svg width={size} height={size} viewBox="0 0 512 512">
          {appIconLanes.map((lane) => (
            <path
              key={lane.d}
              d={lane.d}
              fill="none"
              stroke={appIconColors.lane}
              strokeOpacity={lane.opacity}
              strokeWidth={appIconLaneWidth}
              strokeLinecap="round"
            />
          ))}
        </svg>
      </div>
    ),
    { width: size, height: size },
  );
}
