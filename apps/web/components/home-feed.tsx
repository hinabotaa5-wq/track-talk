"use client";

import { useCallback, useEffect, useLayoutEffect, useRef, useState } from "react";
import { retainHomeFeedWindow, type PostsPage } from "@/lib/fetch-posts";
import { homeFeedPath, nextHomeFeedPath, isFeedFresh, loadHomeFeed, prefetchHomeFeeds, readHomeFeedChunks, saveHomeFeedChunks, homeFeedGeneration } from "@/lib/home-feed-cache";
import { FeedCard } from "@/components/feed-card";
import { MasonryFeed } from "@/components/masonry-feed";
import { FeedLoadingFrame } from "@/components/loading-frame";
import { EmptyState, ErrorState } from "@/components/empty-state";
import { ApiRequestError } from "@/lib/api";
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
  const chunksRef = useRef(chunks);
  useLayoutEffect(() => { chunksRef.current = chunks; });
  const visibleIDs = useRef(new Set<string>());
  const reloading = useRef(new Set<number>());
  const [reloadError, setReloadError] = useState(false);
  const [cursorExpired, setCursorExpired] = useState(false);
  const [loadingMore, setLoadingMore] = useState(false);
  const [moreError, setMoreError] = useState("");
  const morePending = useRef(false);
  const alive = useRef(true);
  const meta: Meta = chunks?.at(-1)?.meta ?? { page, per_page: 12, total: 0 };
  // 重複した投稿は最後のデータと状態を一緒に採用する。
  const postStates = new Map(chunks?.flatMap((chunk) => {
    const unavailable = new Set(chunk.unavailable_ids);
    return chunk.posts.map((post) => [post.id, { post, unloaded: chunk.unloaded, unavailable: unavailable.has(post.id) }] as const);
  }));
  const posts: Post[] | null = chunks ? [...postStates.values()].map((entry) => entry.post) : null;
  const unloadedIDs = new Set([...postStates].filter(([, entry]) => entry.unloaded).map(([id]) => id));
  const unavailableIDs = new Set([...postStates].filter(([, entry]) => entry.unavailable).map(([id]) => id));
  const hasNext = meta.has_more ?? (meta.page * meta.per_page < meta.total);
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
          setChunks((current) => data.meta.has_more !== undefined ? [data] : [data, ...(current?.slice(1) ?? [])]);
          setError("");
          prefetchHomeFeeds(q);
        })
        .catch(() => {
          if (active && !hasInitial) setError(loadError);
        });
    }
    if ((chunksRef.current?.length ?? 0) <= 1 && !(hasInitial && isFeedFresh(path))) refresh(!hasInitial);
    else prefetchHomeFeeds(q);
    function onVisible() {
      if (document.visibilityState === "visible" && (chunksRef.current?.length ?? 0) <= 1 && !isFeedFresh(path)) refresh(false);
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

  const onVisibleIndices = useCallback((indices: number[]) => {
    const current = chunksRef.current;
    if (!current) return;
    const unique = [...new Map(current.flatMap((chunk) => chunk.posts).map((post) => [post.id, post])).values()];
    visibleIDs.current = new Set(indices.map((i) => unique[i]?.id).filter((id): id is string => !!id));
    setChunks((value) => value ? retainHomeFeedWindow(value, visibleIDs.current) : value);
    for (let index = 0; index < current.length; index++) {
      const chunk = current[index];
      if (!chunk.unloaded || reloading.current.has(index) || !chunk.posts.some((post) => visibleIDs.current.has(post.id))) continue;
      reloading.current.add(index);
      const requestPath = chunk.request_path ?? homeFeedPath(kind ?? sort, q, chunk.meta.page);
      loadHomeFeed(requestPath).then((data) => {
        if (!alive.current) return;
        // 順位変動や削除で欠けたIDの枠は残す。既存の座標をずらさない。
        const byID = new Map(data.posts.map((post) => [post.id, post]));
        setChunks((value) => value ? retainHomeFeedWindow(value.map((part, i) => i === index ? {
          ...part, unloaded: false, unavailable_ids: part.posts.filter((post) => !byID.has(post.id)).map((post) => post.id), posts: part.posts.map((post) => byID.get(post.id) ?? { ...post, title: "この投稿は現在表示できません" }),
        } : part), visibleIDs.current) : value);
        reloading.current.delete(index);
      }).catch((error) => {
        if (!alive.current) return;
        if (error instanceof ApiRequestError && error.code === "invalid_cursor") setCursorExpired(true);
        else setReloadError(true);
      });
    }
  }, [kind, sort, q]);

  const loadMore = useCallback(async (retry = false) => {
    if (morePending.current || !hasNext || (moreError && !retry)) return;
    morePending.current = true;
    setLoadingMore(true);
    setMoreError("");
    try {
      const data = await loadHomeFeed(meta.next_cursor ? nextHomeFeedPath(kind ?? sort, q, meta.page + 1, meta.next_cursor) : homeFeedPath(kind ?? sort, q, meta.page + 1));
      if (!alive.current) return;
      // 空のページは終端とし、APIの件数が古くても読み続けない。
      const next = data.posts.length ? data : { ...data, meta: { ...data.meta, total: 0, has_more: false } };
      setChunks((current) => retainHomeFeedWindow([...(current ?? []), next], visibleIDs.current));
    } catch (error) {
      if (alive.current) {
        if (error instanceof ApiRequestError && error.code === "invalid_cursor") setCursorExpired(true);
        setMoreError("続きの読み込みに失敗しました");
      }
    } finally {
      morePending.current = false;
      if (alive.current) setLoadingMore(false);
    }
  }, [hasNext, kind, sort, q, meta.page, meta.next_cursor, moreError]);

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
      {cursorExpired ? <button type="button" className="text-sm text-slate-300" onClick={() => window.location.reload()}>一覧の有効期限が切れました。更新する</button> : null}
      {reloadError ? <button type="button" className="text-sm text-slate-300" onClick={() => { reloading.current.clear(); setReloadError(false); onVisibleIndices(posts.map((post, i) => visibleIDs.current.has(post.id) ? i : -1).filter((i) => i >= 0)); }}>投稿の再取得に失敗しました。再試行</button> : null}
      {posts.length > 0 ? (
        <MasonryFeed onVisibleIndices={onVisibleIndices} compact={posts.map((post) => post.kind === "text" && !post.thumbnail_url)} storageKey={path} onEndReached={hasNext && !moreError ? loadMore : undefined} ratios={[...posts.map((post) => post.youtube_shorts || post.instagram_type === "reel" ? 9 / 16 : 4 / 3), ...(hasNext ? [4 / 3] : [])]}>
          {posts.map((post) => unloadedIDs.has(post.id)
            ? <div key={post.id} aria-busy="true" className="aspect-[4/3] rounded-2xl glass bg-white/10 animate-pulse" />
            : unavailableIDs.has(post.id) ? <div key={post.id} className="flex aspect-[4/3] items-center justify-center rounded-2xl glass p-4 text-sm text-slate-300">この投稿は現在表示できません</div>
            : <FeedCard key={post.id} post={post} />)}
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
