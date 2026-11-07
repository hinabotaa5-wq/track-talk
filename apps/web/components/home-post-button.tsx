"use client";

import { useAuth } from "./auth-provider";
import { Plus } from "lucide-react";
import { MainTabLink } from "./main-tab-link";

// 未ログイン時は投稿画面の先読みや転送を経由せず、直接ログインへ案内する。
// 投稿の入口はアクセントの青で目立たせる。
export function HomePostButton() {
  const { session, sessionLoading } = useAuth();
  return (
    <MainTabLink
      prefetch={true}
      href={!sessionLoading && !session ? "/login?next=/upload" : "/upload"}
      aria-label="投稿する"
      className="inline-flex size-11 shrink-0 items-center justify-center rounded-full border-2 border-[#082554] bg-[#0759c7] text-pure-white shadow-lg shadow-[#020617]/50 transition hover:bg-[#0866df] active:scale-[0.98]"
    >
      <Plus className="size-6" strokeWidth={3} aria-hidden />
    </MainTabLink>
  );
}
