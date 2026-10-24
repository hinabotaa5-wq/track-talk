const storageKey = "tracktalk:last-tab";
export const lastTabChangedEvent = "tracktalk:last-tab-changed";

// 一覧のタブだけを保存する。投稿・ログイン・編集画面や検索語には戻さない。
const tabParams: Record<string, readonly string[]> = {
  "/": ["sort", "type"],
  "/competitions": ["view"],
  "/memo": [],
  "/coaching": [],
  "/injury": ["tab"],
};

// head で即時実行し、React とホームの読み込みを待たずに前回のタブを開く。
// 許可するパスとパラメータは通常の保存処理と共有する。
export const lastTabStartupScript = `(()=>{
  if(location.pathname!=="/"||location.search||location.hash)return;
  try{
    const saved=localStorage.getItem(${JSON.stringify(storageKey)});
    if(!saved)return;
    const [path,search=""]=saved.split("?");
    const tabs=${JSON.stringify(tabParams)};
    if(!Object.hasOwn(tabs,path))return;
    const current=new URLSearchParams(search),params=new URLSearchParams();
    for(const name of tabs[path]){
      const value=current.get(name);
      if(value)params.set(name,value);
    }
    const query=params.toString(),href=path+(query?"?"+query:"");
    if(href==="/")return;
    window.__tracktalkLastTabRedirect=true;
    location.replace(href);
  }catch{window.__tracktalkLastTabRedirect=false;}
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
