"use client";

import { PageLoading } from "@/components/page-loading";

import { useRouter } from "next/navigation";
import { useEffect, useState } from "react";
import { useAuth } from "@/components/auth-provider";
import { FeedCard } from "@/components/feed-card";
import { FeedGrid } from "@/components/feed-grid";
import { ViewSwitch } from "@/components/view-switch";
import { LoadMore } from "@/components/load-more";
import { useCursorPosts } from "@/lib/use-cursor-posts";

export default function MyPostsPage() {
  const { session, token, sessionLoading: loading } = useAuth();
  const router = useRouter();
  const [sort, setSort] = useState<"popular" | "new">("popular");
  // 最初の1ページだけ読み、続きは末尾に近づいたら足す。
  const feed = useCursorPosts(`/v1/me/posts?sort=${sort}`, token);
  const posts = feed.posts ?? [];

  useEffect(() => {
    if (!loading && !session) router.replace("/login?next=/me/posts");
  }, [loading, session, router]);

  if (loading || !session) {
    return <PageLoading label="読み込み中..." />;
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
      {feed.error ? (
        <p className="text-sm text-red-400">{feed.error}</p>
      ) : feed.posts === null ? (
        <PageLoading label="読み込み中..." />
      ) : posts.length === 0 ? (
        <p className="text-sm text-slate-200">まだ投稿がありません</p>
      ) : (
        <FeedGrid>
          {posts.map((p) => (
            <FeedCard key={p.id} post={p} returnTo="me" />
          ))}
        </FeedGrid>
      )}
      {feed.hasMore ? <LoadMore onLoad={() => void feed.loadMore()} loading={feed.loadingMore} error={feed.moreError} expired={feed.expired} /> : null}
    </div>
  );
}
