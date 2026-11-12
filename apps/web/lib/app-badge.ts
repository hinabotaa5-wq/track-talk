// アプリアイコンの通知バッジ（Badging API）。ベルの未読数と同じ数を出す。
// 画面側のバッジ変更は全部ここを通す（Service Worker は単体ファイルのため、
// 同じ検証・握りつぶしの方針の処理を public/sw.js の中に持つ）。
// 未対応の環境では何もせず、例外は握りつぶして通知・画面の本体動作を壊さない。

type BadgeNavigator = {
  setAppBadge?: (count?: number) => Promise<void>;
  clearAppBadge?: () => Promise<void>;
};

function badgeNavigator(): BadgeNavigator | null {
  if (typeof navigator === "undefined") return null;
  return navigator as BadgeNavigator;
}

// 渡す前に検証する。安全な非負整数だけを受け付け、0 はクリア扱いにする。
// null・負数・小数（丸めて 0 になる値を含む）・非数は呼び出し自体をしない。
function normalizeBadgeCount(value: unknown): number | null {
  if (typeof value !== "number" || !Number.isSafeInteger(value) || value < 0) return null;
  return value;
}

export async function applyAppBadge(count: unknown): Promise<void> {
  const normalized = normalizeBadgeCount(count);
  if (normalized === null) return;
  const nav = badgeNavigator();
  if (!nav) return;
  try {
    if (normalized === 0) {
      if (typeof nav.clearAppBadge === "function") await nav.clearAppBadge();
      return;
    }
    if (typeof nav.setAppBadge === "function") await nav.setAppBadge(normalized);
  } catch {
    // バッジが出せなくても画面の動作は変えない。
  }
}

export async function clearAppBadge(): Promise<void> {
  const nav = badgeNavigator();
  if (!nav || typeof nav.clearAppBadge !== "function") return;
  try {
    await nav.clearAppBadge();
  } catch {
    // 消せなくても画面の動作は変えない。
  }
}
