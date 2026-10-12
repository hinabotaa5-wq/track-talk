"use client";

import { useAuth } from "@/components/auth-provider";
import { apiFetch } from "@/lib/api";
import { FeedCard } from "@/components/feed-card";
import type { Meta, Post } from "@/lib/types";
import Link from "next/link";
import { useRouter } from "next/navigation";
import { useEffect, useState } from "react";
import { createClient } from "@/lib/supabase";

export default function MePage() {
  const { session, token, loading } = useAuth();
  const router = useRouter();
  const [posts, setPosts] = useState<Post[]>([]);
  const [postsError, setPostsError] = useState("");

  async function logout() {
    const supabase = createClient();
    await supabase?.auth.signOut();
    router.push("/");
    router.refresh();
  }

  useEffect(() => {
    if (!loading && !session) router.replace("/login");
  }, [loading, session, router]);

  useEffect(() => {
    if (!token) return;
    apiFetch<{ posts: Post[]; meta: Meta }>("/v1/me/posts", { token })
      .then((d) => {
        setPosts(d.posts ?? []);
        setPostsError("");
      })
      .catch(() => setPostsError("投稿の取得に失敗しました"));
  }, [token]);

  if (loading || !session) {
    return <p className="text-sm text-zinc-600">読み込み中...</p>;
  }

  return (
    <div className="space-y-4 md:grid md:grid-cols-[14rem_minmax(0,1fr)] md:items-start md:gap-8 md:space-y-0">
      <div className="space-y-3">
        <h1 className="text-2xl font-bold">マイページ</h1>
        <div className="flex flex-col gap-3 md:rounded-2xl md:border md:bg-white md:p-4">
          <button type="button" onClick={logout} className="block text-left text-sm text-zinc-500">
            ログアウト
          </button>
          <Link href="/me/contact" className="block text-sm text-zinc-500">
            お問い合わせ
          </Link>
          <Link href="/about" className="block text-sm text-zinc-500">
            このアプリについて
          </Link>
          <Link href="/me/delete" className="block text-sm text-red-700">
            退会する
          </Link>
        </div>
      </div>
      <div className="space-y-4">
        <h2 className="text-xl font-semibold">自分の投稿</h2>
        {postsError ? (
          <p className="text-sm text-red-600">{postsError}</p>
        ) : posts.length === 0 ? (
          <p className="text-sm text-zinc-600">まだ投稿がありません</p>
        ) : (
          <div className="grid grid-cols-2 gap-3 lg:grid-cols-3">
            {posts.map((p) => (
              <FeedCard key={p.id} post={p} returnTo="me" />
            ))}
          </div>
        )}
      </div>
    </div>
  );
}
