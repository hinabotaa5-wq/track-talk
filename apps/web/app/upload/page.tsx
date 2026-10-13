"use client";

import Link from "next/link";
import { useAuth } from "@/components/auth-provider";
import { useRouter } from "next/navigation";
import { useEffect } from "react";

const doors = [
  { href: "/posts/new", title: "文章", body: "タイトルと本文でスレッドを立てます。" },
  { href: "/posts/youtube", title: "YouTubeのURL", body: "動画を引用して、そのスレッドに集まります。" },
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

  return (
    <div className="space-y-4">
      <div className="grid gap-3 sm:grid-cols-2 xl:grid-cols-3">
        {doors.map((door) => (
          <Link
            key={door.href}
            href={door.href}
            className="block min-h-11 rounded-2xl border border-white/20 glass bg-white/10 p-4 transition hover:border-blue-700/70 hover:bg-white/16"
          >
            <span className="block text-lg font-semibold text-slate-100">{door.title}</span>
            <span className="mt-1 block text-sm text-slate-200">{door.body}</span>
          </Link>
        ))}
      </div>
    </div>
  );
}
