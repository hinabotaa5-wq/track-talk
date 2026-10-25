"use client";

import { Plus } from "lucide-react";
import { MainTabLink, PendingIcon } from "./main-tab-link";

// 投稿はタブではなく、ホームの左下のボタンから始める。ログインしていなければ proxy がログインへ送る。
// 背景や下のメニュー、選んでいるタブは青なので、投稿の入口は白地に青の＋にして青に埋もれないようにする。
export function PostFab() {
  return (
    <MainTabLink
      prefetch={true}
      href="/upload"
      aria-label="投稿する"
      className="fixed left-4 bottom-[calc(3.5rem+env(safe-area-inset-bottom)+1rem)] z-30 flex size-14 items-center justify-center rounded-full bg-white text-blue-700 shadow-lg shadow-slate-950/60 transition hover:bg-blue-50 active:scale-[0.98] md:left-8 md:bottom-8"
    >
      <PendingIcon className="size-7" label="投稿画面を開いています">
        <Plus className="size-7" strokeWidth={2.4} aria-hidden />
      </PendingIcon>
    </MainTabLink>
  );
}
