"use client";

import { LoadingStopwatch } from "./loading-stopwatch";
import { useIsAnyLinkPending } from "@/lib/pending-link";

// タブやマイページを開くあいだ、いまの画面の上にうっすら幕をかけ、真ん中でストップウォッチの針が回り、空色の輪がたまっていく。
// すぐ開ける移動では出ないよう、CSS で少し待ってからふわっと出す(globals.css の .screen-loader)。
// React が動く前のタップは、起動スクリプトの data-boot-pending を CSS で見て出す。
export function ScreenLoader() {
  const pending = useIsAnyLinkPending();
  return (
    <div className={`screen-loader ${pending ? "flex" : "hidden"}`} aria-hidden={!pending}>
      <LoadingStopwatch />
      <p className="text-xs tracking-[0.12em] text-slate-300">読み込み中</p>
      {pending ? <span role="status" className="sr-only">画面を開いています</span> : null}
    </div>
  );
}
