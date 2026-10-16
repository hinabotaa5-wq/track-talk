"use client";

import Link from "next/link";
import { useAuth } from "@/components/auth-provider";
import { useRouter } from "next/navigation";
import { useEffect } from "react";

const doors = [
  { href: "/posts/new", title: "文章", body: "タイトル・本文・写真のいずれかでスレッドを立てます。" },
  { href: "/posts/url", title: "動画URL", body: "YouTubeやInstagramの動画を引用して、そのスレッドに集まります。" },
  { href: "/posts/video", title: "自分の動画", body: "撮った動画を投稿します。" },
  { href: "/posts/recruit", title: "クラブチームの募集", body: "チーム名や活動場所を書いて、メンバーを募ります。" },
  { href: "/posts/practice", title: "練習相手募集", body: "練習場所と日時を書いて、一緒に練習する相手を募ります。" },
] as const;

export default function UploadPage() {
  const { session, loading } = useAuth();
  const router = useRouter();

  useEffect(() => {
    if (!loading && !session) router.replace("/login");
  }, [loading, session, router]);

  if (loading || !session) return <p className="text-sm text-slate-200">読み込み中…</p>;

  // ホームの「＋」から開く詳細画面。下のメニューは出さないので、その分まで5つの入口で埋める。
  return (
    <div className="flex h-[calc(100dvh-7.125rem-env(safe-area-inset-top)-env(safe-area-inset-bottom))] flex-col gap-3 md:h-auto">
      <h1 className="text-xl font-bold">何を投稿しますか?</h1>
      <div className="grid min-h-0 flex-1 grid-rows-[repeat(5,minmax(0,1fr))] gap-2 md:auto-rows-auto md:grid-cols-2 md:grid-rows-none md:gap-3 xl:grid-cols-3">
        {doors.map((door) => (
          <Link
            key={door.href}
            href={door.href}
            className="flex min-h-0 flex-col justify-center overflow-hidden rounded-2xl border border-white/20 glass bg-white/10 px-3 py-2 transition hover:border-blue-700/70 hover:bg-white/16 active:scale-95"
          >
            <span className="block text-base font-semibold leading-tight text-slate-100">{door.title}</span>
            <span className="mt-0.5 line-clamp-2 text-xs leading-snug text-slate-200">{door.body}</span>
          </Link>
        ))}
      </div>
    </div>
  );
}
