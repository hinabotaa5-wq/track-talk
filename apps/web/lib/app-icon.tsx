import { ImageResponse } from "next/og";

// 仮アイコン。ロゴができたら差し替える。
// maskable でも欠けないよう、文字は中央の安全領域(約 60%)に収める。
export function renderAppIcon(size: number) {
  return new ImageResponse(
    (
      <div
        style={{
          width: "100%",
          height: "100%",
          display: "flex",
          alignItems: "center",
          justifyContent: "center",
          background: "linear-gradient(135deg, #2563eb, #0ea5e9)",
          color: "#ffffff",
          fontSize: size * 0.34,
          fontWeight: 800,
          letterSpacing: -size * 0.01,
        }}
      >
        TT
      </div>
    ),
    { width: size, height: size },
  );
}
