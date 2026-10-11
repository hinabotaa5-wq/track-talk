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
  const { session, profile, loading } = useAuth();
  const router = useRouter();
  const [posts, setPosts] = useState<Post[]>([]);

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
    if (!profile) return;
    apiFetch<{ posts: Post[]; meta: Meta }>(`/v1/users/${profile.id}/posts`).then((d) =>
      setPosts(d.posts ?? []),
    );
  }, [profile]);

  if (!profile) {
    return (
      <div className="space-y-3">
        <h1 className="text-2xl font-bold">マイページ</h1>
        <p>プロフィールが未作成です。</p>
        <Link href="/me/edit" className="text-blue-700">
          プロフィールを作成
        </Link>
        <button type="button" onClick={logout} className="block text-sm text-zinc-500">
          ログアウト
        </button>
        <Link href="/about" className="block text-sm text-zinc-500">
          このアプリについて
        </Link>
      </div>
    );
  }

  return (
    <div className="space-y-4">
      <h1 className="text-2xl font-bold">{profile.nickname}</h1>
      <p className="whitespace-pre-wrap text-zinc-700">{profile.bio || "自己紹介は未設定です。"}</p>
      <Link href="/me/edit" className="text-sm text-blue-700">
        プロフィールを編集
      </Link>
      <button type="button" onClick={logout} className="block text-sm text-zinc-500">
        ログアウト
      </button>
      <Link href="/about" className="block text-sm text-zinc-500">
        このアプリについて
      </Link>
      <h2 className="text-xl font-semibold">自分の投稿</h2>
      {posts.length === 0 ? (
        <p className="text-sm text-zinc-600">まだ投稿がありません</p>
      ) : (
        <div className="grid grid-cols-2 gap-3">
          {posts.map((p) => (
            <FeedCard key={p.id} post={p} />
          ))}
        </div>
      )}
    </div>
  );
}
