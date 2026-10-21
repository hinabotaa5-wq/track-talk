"use client";

import { useRouter } from "next/navigation";
import { useEffect, useState } from "react";
import { useAuth } from "@/components/auth-provider";
import { FeedCard } from "@/components/feed-card";
import { FeedGrid } from "@/components/feed-grid";
import { ViewSwitch } from "@/components/view-switch";
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
      <ViewSwitch
        label="並び順"
        layoutId="my-posts-sort-pill"
        views={[
          { id: "popular", label: "人気" },
          { id: "new", label: "新しい" },
        ]}
        current={sort}
        onSelect={setSort}
      />
      {postsError ? (
        <p className="text-sm text-red-400">{postsError}</p>
      ) : posts.length === 0 ? (
        <p className="text-sm text-slate-200">まだ投稿がありません</p>
      ) : (
        <FeedGrid>
          {posts.map((p) => (
            <FeedCard key={p.id} post={p} returnTo="me" />
          ))}
        </FeedGrid>
      )}
    </div>
  );
}
