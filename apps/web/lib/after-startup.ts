// 起動中の通信・描画と競合しないよう、先読みや計測はページの読み込み後の空き時間へ回す。
export function afterStartup(callback: () => void): () => void {
  let timer: ReturnType<typeof setTimeout> | undefined;
  let idle: number | undefined;
  function schedule() {
    timer = setTimeout(() => {
      if (window.requestIdleCallback) {
        idle = window.requestIdleCallback(callback, { timeout: 2_000 });
      } else {
        callback();
      }
    }, 1_500);
  }
  if (document.readyState === "complete") schedule();
  else window.addEventListener("load", schedule, { once: true });
  return () => {
    window.removeEventListener("load", schedule);
    clearTimeout(timer);
    if (idle !== undefined) window.cancelIdleCallback(idle);
  };
}
