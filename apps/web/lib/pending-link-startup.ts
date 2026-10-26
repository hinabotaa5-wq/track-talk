// layout(サーバー)から読むので、React のフックを使う pending-link.ts とは分ける。
// React が動き出す前も、リンクの待ち時間を超えた場合だけクルクルを出す。
// その間のリンクはブラウザがそのまま開くので、押されたリンクに印を付けて CSS で切り替える。
export const pendingLinkStartupScript = `(()=>{
  let timer;
  document.addEventListener("click",(e)=>{
    if(window.__tracktalkHydrated||e.defaultPrevented||e.button!==0||e.metaKey||e.ctrlKey||e.shiftKey||e.altKey)return;
    const link=e.target instanceof Element?e.target.closest("a[data-pending-link]"):null;
    if(!link||link.target==="_blank")return;
    const url=new URL(link.href,location.href);
    if(url.origin!==location.origin||url.pathname+url.search===location.pathname+location.search)return;
    clearTimeout(timer);
    document.querySelectorAll("[data-boot-pending]").forEach((item)=>item.removeAttribute("data-boot-pending"));
    const delay=Number(link.dataset.pendingDelay)||0;
    if(delay)timer=setTimeout(()=>{if(!window.__tracktalkHydrated&&link.isConnected)link.setAttribute("data-boot-pending","");},delay);
    else link.setAttribute("data-boot-pending","");
  });
  addEventListener("pagehide",()=>clearTimeout(timer));
  addEventListener("pageshow",(e)=>{
    clearTimeout(timer);
    if(e.persisted)document.querySelectorAll("[data-boot-pending]").forEach((link)=>link.removeAttribute("data-boot-pending"));
  });
})();`;
