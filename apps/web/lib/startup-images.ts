import screens from "./startup-screens.json";

// 画像は scripts/generate-startup-assets.cjs でロゴの定義から生成する。
export const startupImages = screens.flatMap(([width, height, ratio]) =>
  (["portrait", "landscape"] as const).map((orientation) => {
    const landscape = orientation === "landscape";
    const pixelWidth = (landscape ? height : width) * ratio;
    const pixelHeight = (landscape ? width : height) * ratio;
    return {
      url: `/startup/${pixelWidth}x${pixelHeight}.png`,
      media: `(device-width: ${width}px) and (device-height: ${height}px) and (-webkit-device-pixel-ratio: ${ratio}) and (orientation: ${orientation})`,
    };
  }),
);
