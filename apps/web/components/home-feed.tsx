"use client";

import { useCallback, useEffect, useRef, useState } from "react";
import type { PostsPage } from "@/lib/fetch-posts";
import { homeFeedPath, isFeedFresh, loadHomeFeed, prefetchHomeFeeds, readHomeFeedChunks, saveHomeFeedChunks, homeFeedGeneration } from "@/lib/home-feed-cache";
import { FeedCard } from "@/components/feed-card";
import { MasonryFeed } from "@/components/masonry-feed";
import { FeedLoadingFrame } from "@/components/loading-frame";
import { EmptyState, ErrorState } from "@/components/empty-state";
import type { Meta, Post } from "@/lib/types";

const loadError = "投稿の取得に失敗しました。APIの起動を確認してください。";

export function HomeFeed({
  sort,
  q,
  page,
  initial,
  kind,
}: {
  sort: "popular" | "new";
  q: string;
  page: number;
  initial: PostsPage | null;
  kind?: "recruitment";
}) {
  const path = homeFeedPath(kind ?? sort, q, page);
  const [chunks, setChunks] = useState<PostsPage[] | null>(() => readHomeFeedChunks(path) ?? (initial ? [initial] : null));
  const [loadingMore, setLoadingMore] = useState(false);
  const [moreError, setMoreError] = useState("");
  const morePending = useRef(false);
  const alive = useRef(true);
  const meta: Meta = chunks?.at(-1)?.meta ?? { page, per_page: 12, total: 0 };
  const posts: Post[] | null = chunks ? [...new Map(chunks.flatMap((chunk) => chunk.posts).map((post) => [post.id, post])).values()] : null;
  const hasNext = meta.page * meta.per_page < meta.total;
  const [error, setError] = useState("");
  const hasInitial = initial !== null;

  useEffect(() => {
    let active = true;
    alive.current = true;
    function refresh(retry: boolean) {
      // データがなければ API が起きるまで読み直し続ける。古いデータがあれば、目立たないように更新する。
      loadHomeFeed(path, retry)
        .then((data) => {
          if (!active) return;
          setChunks((current) => [data, ...(current?.slice(1) ?? [])]);
          setError("");
          prefetchHomeFeeds(q);
        })
        .catch(() => {
          if (active && !hasInitial) setError(loadError);
        });
    }
    if (!(hasInitial && isFeedFresh(path))) refresh(!hasInitial);
    else prefetchHomeFeeds(q);
    function onVisible() {
      if (document.visibilityState === "visible") refresh(false);
    }
    document.addEventListener("visibilitychange", onVisible);
    return () => {
      active = false;
      alive.current = false;
      document.removeEventListener("visibilitychange", onVisible);
    };
  }, [hasInitial, path, q]);

  useEffect(() => {
    // 全ページを残し、詳細から戻ったときに後から読み込んだ投稿も復元する。
    const atGeneration = homeFeedGeneration();
    return () => { if (chunks) saveHomeFeedChunks(path, chunks, atGeneration); };
  }, [chunks, path]);

  const loadMore = useCallback(async (retry = false) => {
    if (morePending.current || !hasNext || (moreError && !retry)) return;
    morePending.current = true;
    setLoadingMore(true);
    setMoreError("");
    try {
      const data = await loadHomeFeed(homeFeedPath(kind ?? sort, q, meta.page + 1));
      if (!alive.current) return;
      // 空のページは終端とし、APIの件数が古くても読み続けない。
      const next = data.posts.length ? data : { ...data, meta: { ...data.meta, total: 0 } };
      setChunks((current) => [...(current ?? []), next]);
    } catch {
      if (alive.current) setMoreError("続きの読み込みに失敗しました");
    } finally {
      morePending.current = false;
      if (alive.current) setLoadingMore(false);
    }
  }, [hasNext, kind, sort, q, meta.page, moreError]);

  if (error) return <ErrorState message={error} />;
  if (posts === null) return <FeedLoadingFrame horizontal />;

  return (
    <>
      {posts.length === 0 ? (
        <EmptyState
          title={q ? "一致する投稿がありません" : kind === "recruitment" ? "まだ募集がありません" : "まだ投稿がありません"}
          body={q ? "別の言葉で探してください。" : kind === "recruitment" ? undefined : "最初のレース感想を書いてみましょう。"}
        />
      ) : null}
      {posts.length > 0 ? (
        <MasonryFeed storageKey={path} onEndReached={hasNext && !moreError ? loadMore : undefined} ratios={[...posts.map((post) => post.youtube_shorts || post.instagram_type === "reel" ? 9 / 16 : 4 / 3), ...(hasNext ? [4 / 3] : [])]}>
          {posts.map((post) => <FeedCard key={post.id} post={post} />)}
          {hasNext ? (
            <button
              key="next-page"
              type="button"
              onClick={() => void loadMore(true)}
              disabled={loadingMore}
              aria-live="polite"
              className="masonry-feed-next flex aspect-[4/3] w-full items-center justify-center rounded-2xl border border-white/20 glass bg-white/10 px-4 text-center text-sm font-semibold text-slate-200 transition hover:bg-white/16 disabled:opacity-60"
            >
              {loadingMore ? "読み込み中…" : moreError ? "読み込みに失敗しました。再試行" : "続きを読み込む"}
            </button>
          ) : null}
        </MasonryFeed>
      ) : null}
    </>
  );
}
