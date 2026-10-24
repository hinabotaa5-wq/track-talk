const storageKey = "tracktalk:last-tab";

// 一覧のタブだけを保存する。投稿・ログイン・編集画面や検索語には戻さない。
const tabParams: Record<string, readonly string[]> = {
  "/": ["sort", "type"],
  "/competitions": ["view"],
  "/memo": [],
  "/coaching": [],
  "/injury": ["tab"],
};

export function lastTabHref(pathname: string, search: string) {
  if (!Object.hasOwn(tabParams, pathname)) return null;
  const current = new URLSearchParams(search);
  const params = new URLSearchParams();
  for (const name of tabParams[pathname]) {
    const value = current.get(name);
    if (value) params.set(name, value);
  }
  const query = params.toString();
  return query ? `${pathname}?${query}` : pathname;
}

export function readLastTab() {
  try {
    const saved = localStorage.getItem(storageKey);
    if (!saved) return null;
    const [pathname, search = ""] = saved.split("?");
    // 保存値が壊れていても、外部サイトや任意のページには移動しない。
    return lastTabHref(pathname, search);
  } catch {
    return null;
  }
}

export function rememberLastTab(pathname: string, search: string) {
  const href = lastTabHref(pathname, search);
  if (!href) return;
  try { localStorage.setItem(storageKey, href); } catch { /* 保存不可でもタブは使える。 */ }
}
