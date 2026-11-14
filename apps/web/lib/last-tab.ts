export const lastTabStorageKey = "tracktalk:last-tab";
const storageKey = lastTabStorageKey;
export const lastTabChangedEvent = "tracktalk:last-tab-changed";

// 一覧のタブだけを保存する。投稿・ログイン・編集画面や検索語には戻さない。
const tabParams: Record<string, readonly string[]> = {
  "/": ["sort", "type"],
  "/competitions": ["view"],
  "/memo": [],
  "/coaching": [],
  "/injury": ["tab"],
};

// 起動ロゴの直後で実行し、React とホームの読み込みを待たずにコーチタブを開く。
// 保存値は読まない。素の "/" なら保存内容にかかわらず "/coaching" へ移動する。
export const lastTabStartupScript = `(()=>{
  if(location.pathname!=="/"||location.search||location.hash)return;
  window.__tracktalkLastTabRedirect=true;
  const go=()=>location.replace("/coaching");
  if(typeof requestAnimationFrame==="function"){
    requestAnimationFrame(()=>requestAnimationFrame(go));
  }else{go();}
})();`;

export function isLastTabStartupRedirecting() {
  return !!(window as Window & { __tracktalkLastTabRedirect?: boolean }).__tracktalkLastTabRedirect;
}

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

// 素の "/" の冷起動での解決。保存値（大会・メモ・ケガ・コーチ・フィード・
// 保存なし・不正値・保存不可）にかかわらず "/coaching" へ移動する。
// 保存値は読まない。色付け用の保存・読み出し（rememberLastTab／readLastTab）は残す。
export function resolveColdStartTarget() {
  return "/coaching";
}

export function rememberLastTab(pathname: string, search: string) {
  const href = lastTabHref(pathname, search);
  if (!href) return;
  try {
    if (localStorage.getItem(storageKey) !== href) {
      localStorage.setItem(storageKey, href);
      window.dispatchEvent(new Event(lastTabChangedEvent));
    }
  } catch { /* 保存不可でもタブは使える。 */ }
}
