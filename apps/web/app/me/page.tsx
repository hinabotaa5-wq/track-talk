"use client";

import { useAuth } from "@/components/auth-provider";
import { apiFetch } from "@/lib/api";
import { FeedCard } from "@/components/feed-card";
import type { Meta, Post } from "@/lib/types";
import Link from "next/link";
import { useRouter } from "next/navigation";
import { useEffect, useState } from "react";
import { createClient } from "@/lib/supabase";
import { useActionLock } from "@/lib/use-action-lock";

export default function MePage() {
  const { session, token, loading } = useAuth();
  const router = useRouter();
  const [posts, setPosts] = useState<Post[]>([]);
  const [postsError, setPostsError] = useState("");

  const logoutLock = useActionLock();

  async function logout() {
    if (!logoutLock.acquire()) return;
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
    return <p className="text-sm text-slate-200">読み込み中...</p>;
  }

  return (
    <div className="space-y-4 md:grid md:grid-cols-[14rem_minmax(0,1fr)] md:items-start md:gap-8 md:space-y-0">
      <div className="space-y-3">
        <div className="flex flex-col gap-3 md:rounded-2xl md:border md:border-white/20 md:bg-white/10 md:p-4 md:backdrop-blur-2xl">
          <button
            type="button"
            onClick={logout}
            disabled={logoutLock.pending}
            className="block text-left text-sm text-slate-200 disabled:opacity-50"
          >
            ログアウト
          </button>
          <Link href="/me/contact" className="block text-sm text-slate-200">
            お問い合わせ
          </Link>
          <Link href="/guidelines" className="block text-sm text-slate-200">
            ガイドライン
          </Link>
          <Link href="/terms" className="block text-sm text-slate-200">
            利用規約
          </Link>
          <Link href="/privacy" className="block text-sm text-slate-200">
            プライバシーポリシー
          </Link>
          <Link href="/me/delete" className="block text-sm text-red-400">
            退会する
          </Link>
        </div>
      </div>
      <div className="space-y-4">
        <h2 className="text-xl font-semibold">自分の投稿</h2>
        {postsError ? (
          <p className="text-sm text-red-400">{postsError}</p>
        ) : posts.length === 0 ? (
          <p className="text-sm text-slate-200">まだ投稿がありません</p>
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
