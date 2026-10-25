// layout(サーバー)から読むので、React のフックを使う pending-link.ts とは分ける。
// React が動き出す前(アプリを開いた直後)のタップでも、すぐにクルクルを出す。
// その間のリンクはブラウザがそのまま開くので、押されたリンクに印を付けて CSS で切り替える。
export const pendingLinkStartupScript = `(()=>{
  document.addEventListener("click",(e)=>{
    if(window.__tracktalkHydrated||e.defaultPrevented||e.button!==0||e.metaKey||e.ctrlKey||e.shiftKey||e.altKey)return;
    const link=e.target instanceof Element?e.target.closest("a[data-pending-link]"):null;
    if(!link||link.target==="_blank")return;
    const url=new URL(link.href,location.href);
    if(url.origin!==location.origin||url.pathname+url.search===location.pathname+location.search)return;
    link.setAttribute("data-boot-pending","");
  });
  addEventListener("pageshow",(e)=>{
    if(e.persisted)document.querySelectorAll("[data-boot-pending]").forEach((link)=>link.removeAttribute("data-boot-pending"));
  });
})();`;
