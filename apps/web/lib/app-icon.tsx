import { ImageResponse } from "next/og";
import { appIconColors } from "@/components/app-icon-mark";

// ヘッダーのロゴ(トラックの3レーン)をアプリアイコンにする。
// ホーム画面で沈まないよう、背景は明るい青のグラデーションにして、真ん中のレーンは濃い紺で引き締める。
// 色は components/app-icon-mark.tsx(appIconColors)と揃える。
// maskable でも欠けないよう、マークは中央の安全領域(直径 80% の円)に収める。
export function renderAppIcon(size: number, markScale = 0.56) {
  const markWidth = size * markScale;
  const markHeight = (markWidth * 38) / 42;
  return new ImageResponse(
    (
      <div
        style={{
          width: "100%",
          height: "100%",
          display: "flex",
          alignItems: "center",
          justifyContent: "center",
          backgroundImage: `linear-gradient(135deg, ${appIconColors.from}, ${appIconColors.to})`,
        }}
      >
        <svg width={markWidth} height={markHeight} viewBox="4 6 42 38">
          <path
            d="M42 10H28A20 20 0 0 0 8 30v10"
            fill="none"
            stroke={appIconColors.outer}
            strokeWidth="2.6"
            strokeLinecap="round"
            strokeLinejoin="round"
          />
          <path
            d="M42 16.5H28A13.5 13.5 0 0 0 14.5 30v10"
            fill="none"
            stroke={appIconColors.middle}
            strokeWidth="2.6"
            strokeLinecap="round"
            strokeLinejoin="round"
          />
          <path
            d="M42 23H28A7 7 0 0 0 21 30v10"
            fill="none"
            stroke={appIconColors.inner}
            strokeWidth="2.6"
            strokeLinecap="round"
            strokeLinejoin="round"
          />
        </svg>
      </div>
    ),
    { width: size, height: size },
  );
}
