"use client";

import Link from "next/link";
import { useAuth } from "@/components/auth-provider";
import { useRouter } from "next/navigation";
import { useEffect } from "react";

const doors = [
  { href: "/posts/new", title: "文章", body: "タイトルと本文でスレを立てます。" },
  { href: "/posts/youtube", title: "YouTubeのURL", body: "動画を引用して、そのスレに集まります。" },
  { href: "/posts/video", title: "自分の動画", body: "撮った動画を投稿します。" },
] as const;

export default function UploadPage() {
  const { session, loading } = useAuth();
  const router = useRouter();

  useEffect(() => {
    if (!loading && !session) router.replace("/login");
  }, [loading, session, router]);

  if (loading || !session) return <p>読み込み中…</p>;

  return (
    <div className="mx-auto max-w-2xl space-y-4">
      <h1 className="text-2xl font-bold">投稿</h1>
      <p className="text-sm text-zinc-600">文章、YouTube、自分の動画から選べます。</p>
      <div className="space-y-3">
        {doors.map((door) => (
          <Link
            key={door.href}
            href={door.href}
            className="block min-h-11 rounded-2xl border bg-white p-4 hover:border-blue-300"
          >
            <span className="block text-lg font-semibold">{door.title}</span>
            <span className="mt-1 block text-sm text-zinc-600">{door.body}</span>
          </Link>
        ))}
      </div>
    </div>
  );
}
