"use client";

import { useIsAnyLinkPending } from "@/lib/pending-link";

// タブやマイページを開くあいだ、いまの画面の上にうっすら幕をかけ、真ん中で楕円のトラックを光が周回する。
// すぐ開ける移動では出ないよう、CSS で少し待ってからふわっと出す(globals.css の .screen-loader)。
// React が動く前のタップは、起動スクリプトの data-boot-pending を CSS で見て出す。
export function ScreenLoader() {
  const pending = useIsAnyLinkPending();
  return (
    <div className={`screen-loader ${pending ? "flex" : "hidden"}`} aria-hidden={!pending}>
      <svg className="screen-loader-track" viewBox="0 0 150 84" aria-hidden>
        <rect className="screen-loader-lane" x="8" y="8" width="134" height="68" rx="34" />
        <rect className="screen-loader-line" x="8" y="8" width="134" height="68" rx="34" />
        <rect className="screen-loader-runner" x="8" y="8" width="134" height="68" rx="34" pathLength="400" />
      </svg>
      <p className="text-xs tracking-[0.12em] text-slate-300">読み込み中</p>
      {pending ? <span role="status" className="sr-only">画面を開いています</span> : null}
    </div>
  );
}
