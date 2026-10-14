"use client";

import Link from "next/link";
import { useEffect, useState } from "react";
import { useRouter } from "next/navigation";
import { useAuth } from "@/components/auth-provider";
import { loadSavedFeed, prefetchHomeFeeds, readFeed, savedFeedPath } from "@/lib/home-feed-cache";
import { FeedCard, feedGridClass } from "@/components/feed-card";
import { FeedLoadingFrame } from "@/components/loading-frame";
import { EmptyState, ErrorState } from "@/components/empty-state";
import type { Meta, Post } from "@/lib/types";

export function SavedFeed({ q, page }: { q: string; page: number }) {
  const { session, loading, token } = useAuth();
  const router = useRouter();
  const path = savedFeedPath(q, page);
  const [cached] = useState(() => readFeed(path));
  const [posts, setPosts] = useState<Post[]>(cached?.posts ?? []);
  const [meta, setMeta] = useState<Meta>(cached?.meta ?? { page: 1, per_page: 20, total: 0 });
  const [error, setError] = useState("");
  const [ready, setReady] = useState(cached !== undefined);

  useEffect(() => {
    if (!loading && !session) router.replace("/login");
  }, [loading, session, router]);

  useEffect(() => {
    if (!token) return;
    let active = true;
    // Cached saves show at once; always refresh because saving happens on other screens.
    loadSavedFeed(path, token)
      .then((data) => {
        if (!active) return;
        setPosts(data.posts);
        setMeta(data.meta);
        setError("");
        prefetchHomeFeeds(q);
      })
      .catch(() => {
        if (active && readFeed(path) === undefined) setError("保存した投稿の取得に失敗しました。");
      })
      .finally(() => {
        if (active) setReady(true);
      });
    return () => {
      active = false;
    };
  }, [token, path, q]);

  if (loading || !session || !ready) return <FeedLoadingFrame />;

  const hasNext = meta.page * meta.per_page < meta.total;
  const next = new URLSearchParams();
  next.set("sort", "saved");
  if (q) next.set("q", q);
  next.set("page", String(meta.page + 1));

  return (
    <>
      {error ? <ErrorState message={error} /> : null}
      {!error && posts.length === 0 ? (
        <EmptyState
          title={q ? "一致する投稿がありません" : "まだ保存がありません"}
          body={q ? "別の言葉で探してください。" : "投稿の保存ボタンから残せます。"}
        />
      ) : null}
      {posts.length > 0 ? (
        <div className={feedGridClass}>
          {posts.map((post) => (
            <FeedCard key={post.id} post={post} />
          ))}
        </div>
      ) : null}
      {hasNext ? (
        <Link href={`/?${next}`} className="block w-full rounded-full border border-white/20 glass bg-white/10 py-3 text-center text-sm font-semibold text-slate-200 shadow-xs transition hover:bg-white/16 hover:text-white active:scale-95">
          次の投稿
        </Link>
      ) : null}
    </>
  );
}
