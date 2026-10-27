"use client";

import { useIsAnyLinkPending } from "@/lib/pending-link";

// タブやマイページを開くあいだ、いまの画面の上にうっすら幕をかけ、真ん中でストップウォッチの針が回り、空色の輪がたまっていく。
// すぐ開ける移動では出ないよう、CSS で少し待ってからふわっと出す(globals.css の .screen-loader)。
// React が動く前のタップは、起動スクリプトの data-boot-pending を CSS で見て出す。
export function ScreenLoader() {
  const pending = useIsAnyLinkPending();
  return (
    <div className={`screen-loader ${pending ? "flex" : "hidden"}`} aria-hidden={!pending}>
      <svg className="screen-loader-watch" viewBox="0 0 86 96" aria-hidden>
        <rect className="screen-loader-crown" x="37" y="0" width="12" height="7" rx="2" />
        <rect className="screen-loader-crown" x="41" y="6" width="4" height="6" />
        <circle className="screen-loader-face" cx="43" cy="54" r="36" />
        <circle className="screen-loader-arc" cx="43" cy="54" r="32" pathLength="100" />
        <path className="screen-loader-tick" d="M43 24v5M73 54h-5M43 84v-5M13 54h5" />
        <line className="screen-loader-hand" x1="43" y1="54" x2="43" y2="28" />
        <circle className="screen-loader-pin" cx="43" cy="54" r="3.5" />
      </svg>
      <p className="text-xs tracking-[0.12em] text-slate-300">読み込み中</p>
      {pending ? <span role="status" className="sr-only">画面を開いています</span> : null}
    </div>
  );
}
