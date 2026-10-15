export type InstallPlatform = "ios-safari" | "ios-other" | "android" | "other";

export const installGuideDismissMs = 7 * 24 * 60 * 60 * 1000;
export const installGuideStorageKey = "tracktalk:install-guide-dismissed-at";
export const installGuideOpenEvent = "tracktalk:install-guide-open";

// iPadOS は UA が Macintosh になるため、タッチ点数で見分ける。
function isIos(ua: string, maxTouchPoints: number) {
  return /iPhone|iPad|iPod/.test(ua) || (/Macintosh/.test(ua) && maxTouchPoints > 1);
}

export function detectInstallPlatform(ua: string, maxTouchPoints = 0): InstallPlatform {
  if (isIos(ua, maxTouchPoints)) {
    // Safari 以外(Chrome / Firefox / Edge / アプリ内ブラウザ)では「ホーム画面に追加」の手順が違う。
    const other = /CriOS|FxiOS|EdgiOS|OPiOS|GSA\/|Line\/|Instagram|FBAN|FBAV|Twitter/.test(ua);
    return other ? "ios-other" : "ios-safari";
  }
  if (/Android/.test(ua)) return "android";
  return "other";
}

export function isDismissed(dismissedAt: number | null, now: number) {
  if (dismissedAt === null || !Number.isFinite(dismissedAt)) return false;
  return now - dismissedAt < installGuideDismissMs;
}
