import { ImageResponse } from "next/og";

// ヘッダーのロゴ(トラックの3レーン)をアプリアイコンにする。
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
          background: "#16243a",
        }}
      >
        <svg width={markWidth} height={markHeight} viewBox="4 6 42 38">
          <path
            d="M42 10H28A20 20 0 0 0 8 30v10"
            fill="none"
            stroke="#ffffff"
            strokeWidth="2.6"
            strokeLinecap="round"
            strokeLinejoin="round"
          />
          <path
            d="M42 16.5H28A13.5 13.5 0 0 0 14.5 30v10"
            fill="none"
            stroke="#93c5fd"
            strokeWidth="2.6"
            strokeLinecap="round"
            strokeLinejoin="round"
          />
          <path
            d="M42 23H28A7 7 0 0 0 21 30v10"
            fill="none"
            stroke="rgba(255,255,255,0.8)"
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
