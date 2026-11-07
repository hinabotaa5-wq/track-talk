"use client";

import { useAuth } from "./auth-provider";
import { Plus } from "lucide-react";
import { useSearchParams } from "next/navigation";
import { parseHomeSort } from "@/lib/home-sort";
import { MainTabLink } from "./main-tab-link";

// 未ログイン時は投稿画面の先読みや転送を経由せず、直接ログインへ案内する。
// 投稿の入口はアクセントの青で目立たせる。
export function PostFab() {
  const { session, sessionLoading } = useAuth();
  return (
    <MainTabLink
      prefetch={true}
      href={!sessionLoading && !session ? "/login?next=/upload" : "/upload"}
      aria-label="投稿する"
      className="home-post-fab fixed left-4 z-45 flex size-14 items-center justify-center rounded-full border-2 border-[#082554] bg-[#0759c7] text-pure-white shadow-lg shadow-[#020617]/50 transition hover:bg-[#0866df] active:scale-[0.98] md:left-8"
    >
      <Plus className="size-7" strokeWidth={3} aria-hidden />
    </MainTabLink>
  );
}

// レビューのタブでは投稿ボタンを出さない。
export function HomePostFab() {
  const sp = useSearchParams();
  if (parseHomeSort(sp.get("sort")) === "review") return null;
  return <PostFab />;
}
