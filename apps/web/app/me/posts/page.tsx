"use client";

import { useRouter } from "next/navigation";
import { useEffect, useState } from "react";
import { useAuth } from "@/components/auth-provider";
import { FeedCard, feedGridClass } from "@/components/feed-card";
import { apiFetch } from "@/lib/api";
import { fetchAllPages } from "@/lib/paging";
import type { Meta, Post } from "@/lib/types";

export default function MyPostsPage() {
  const { session, token, loading } = useAuth();
  const router = useRouter();
  const [posts, setPosts] = useState<Post[]>([]);
  const [postsError, setPostsError] = useState("");
  const [sort, setSort] = useState<"popular" | "new">("popular");

  useEffect(() => {
    if (!loading && !session) router.replace("/login?next=/me/posts");
  }, [loading, session, router]);

  useEffect(() => {
    if (!token) return;
    let active = true;
    // API は1ページ20件（最大50件）で切るので、最後のページまで集める。
    fetchAllPages(async (page) => {
      const d = await apiFetch<{ posts: Post[]; meta: Meta }>(`/v1/me/posts?sort=${sort}&page=${page}&per_page=50`, { token });
      return { items: d.posts ?? [], total: d.meta.total };
    })
      .then((d) => {
        if (!active) return;
        setPosts(d.items);
        setPostsError("");
      })
      .catch(() => {
        if (active) setPostsError("投稿の取得に失敗しました");
      });
    return () => {
      active = false;
    };
  }, [token, sort]);

  if (loading || !session) {
    return <p className="text-sm text-slate-200">読み込み中...</p>;
  }

  return (
    <div className="space-y-4">
      <h1 className="text-2xl font-bold">自分の投稿</h1>
      <div className="flex flex-wrap gap-2">
        {(
          [
            ["popular", "人気"],
            ["new", "新しい"],
          ] as const
        ).map(([value, label]) => (
          <button
            key={value}
            type="button"
            onClick={() => setSort(value)}
            aria-pressed={sort === value}
            className={`inline-flex min-h-11 cursor-pointer items-center rounded-full border px-4.5 text-sm transition-all duration-200 active:scale-95 ${
              sort === value
                ? "border-white/35 bg-blue-500/75 font-semibold text-white shadow-md shadow-blue-950/30 backdrop-blur-xl"
                : "glass border-white/20 bg-white/10 font-medium text-slate-300 hover:bg-white/16 hover:text-white"
            }`}
          >
            {label}
          </button>
        ))}
      </div>
      {postsError ? (
        <p className="text-sm text-red-400">{postsError}</p>
      ) : posts.length === 0 ? (
        <p className="text-sm text-slate-200">まだ投稿がありません</p>
      ) : (
        <div className={feedGridClass}>
          {posts.map((p) => (
            <FeedCard key={p.id} post={p} returnTo="me" />
          ))}
        </div>
      )}
    </div>
  );
}
