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

// 起動ロゴの直後で実行し、React とホームの読み込みを待たずに前回のタブを開く。
// 許可するパスとパラメータは通常の保存処理と共有する。
export const lastTabStartupScript = `(()=>{
  if(location.pathname!=="/"||location.search||location.hash)return;
  try{
    const saved=localStorage.getItem(${JSON.stringify(storageKey)});
    const tabs=${JSON.stringify(tabParams)};
    const navigate=(href)=>{
      window.__tracktalkLastTabRedirect=true;
      const go=()=>location.replace(href);
      if(typeof requestAnimationFrame==="function"){
        requestAnimationFrame(()=>requestAnimationFrame(go));
      }else{go();}
    };
    if(!saved){navigate("/coaching");return;}
    const [path,search=""]=saved.split("?");
    if(!Object.hasOwn(tabs,path))return;
    if(path==="/"){navigate("/coaching");return;}
    const current=new URLSearchParams(search),params=new URLSearchParams();
    for(const name of tabs[path]){
      const value=current.get(name);
      if(value)params.set(name,value);
    }
    const query=params.toString(),href=path+(query?"?"+query:"");
    navigate(href);
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

// 素の "/" の冷起動での解決（案B）。保存なし・保存値 "/" は "/coaching"、
// 不正な保存値・保存不可は移動しない（null）。
export function resolveColdStartTarget() {
  let saved: string | null;
  try {
    saved = localStorage.getItem(storageKey);
  } catch {
    return null;
  }
  if (!saved) return "/coaching";
  let href: string | null;
  try {
    const [pathname, search = ""] = saved.split("?");
    href = lastTabHref(pathname, search);
  } catch {
    return null;
  }
  if (!href) return null;
  if (href === "/" || href.startsWith("/?")) return "/coaching";
  return href;
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
