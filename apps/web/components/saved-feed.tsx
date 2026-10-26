"use client";

import { useEffect } from "react";
import { useRouter } from "next/navigation";
import { useAuth } from "@/components/auth-provider";
import { FeedCard } from "@/components/feed-card";
import { FeedGrid } from "@/components/feed-grid";
import { FeedLoadingFrame } from "@/components/loading-frame";
import { EmptyState, ErrorState } from "@/components/empty-state";
import { LoadMore } from "@/components/load-more";
import { useCursorPosts } from "@/lib/use-cursor-posts";

export function SavedFeed() {
  const { session, sessionLoading, token } = useAuth();
  const router = useRouter();
  const feed = useCursorPosts("/v1/me/saves", token);
  useEffect(() => {
    if (!sessionLoading && !session) router.replace("/login?next=/me/saved");
  }, [sessionLoading, session, router]);
  if (sessionLoading || !session || feed.posts === null) return <FeedLoadingFrame />;
  return <>
    {feed.error ? <ErrorState message={feed.error} /> : null}
    {!feed.error && !feed.posts.length ? <EmptyState title="まだ保存がありません" body="投稿の保存ボタンから残せます。" /> : null}
    <FeedGrid>{feed.posts.map((post) => <FeedCard key={post.id} post={post} />)}</FeedGrid>
    {feed.hasMore ? <LoadMore onLoad={() => void feed.loadMore()} loading={feed.loadingMore} error={feed.moreError} expired={feed.expired} /> : null}
  </>;
}
