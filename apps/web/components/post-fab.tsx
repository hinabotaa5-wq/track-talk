import Link from "next/link";
import { Plus } from "lucide-react";

// 投稿はタブではなく、ホームの右下のボタンから始める。ログインしていなければ proxy がログインへ送る。
// 背景や下のメニュー、選んでいるタブは青なので、投稿の入口はオレンジにして青に埋もれないようにする。
export function PostFab() {
  return (
    <Link
      href="/upload"
      aria-label="投稿する"
      className="fixed right-4 bottom-[calc(3.5rem+env(safe-area-inset-bottom)+1rem)] z-30 flex size-14 items-center justify-center rounded-full bg-orange-500 text-white shadow-lg shadow-orange-950/50 ring-2 ring-white/70 transition hover:bg-orange-400 active:scale-95 md:right-8 md:bottom-8"
    >
      <Plus className="size-7" strokeWidth={2.4} aria-hidden />
    </Link>
  );
}
