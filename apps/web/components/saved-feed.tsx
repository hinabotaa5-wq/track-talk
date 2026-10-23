"use client";

import Link from "next/link";
import { useEffect, useState } from "react";
import { useRouter } from "next/navigation";
import { useAuth } from "@/components/auth-provider";
import { loadSavedFeed, readFeed, savedFeedPath } from "@/lib/home-feed-cache";
import { FeedCard } from "@/components/feed-card";
import { FeedGrid } from "@/components/feed-grid";
import { FeedLoadingFrame } from "@/components/loading-frame";
import { EmptyState, ErrorState } from "@/components/empty-state";
import type { Meta, Post } from "@/lib/types";

export function SavedFeed({ page }: { page: number }) {
  const { session, loading, token } = useAuth();
  const router = useRouter();
  const path = savedFeedPath("", page);
  const [cached] = useState(() => readFeed(path));
  const [posts, setPosts] = useState<Post[]>(cached?.posts ?? []);
  const [meta, setMeta] = useState<Meta>(cached?.meta ?? { page: 1, per_page: 20, total: 0 });
  const [error, setError] = useState("");
  const [ready, setReady] = useState(cached !== undefined);

  useEffect(() => {
    if (!loading && !session) router.replace("/login?next=/me/saved");
  }, [loading, session, router]);

  useEffect(() => {
    if (!token) return;
    let active = true;
    // キャッシュした保存はすぐに出す。保存はほかの画面でもされるので、いつも読み直す。
    loadSavedFeed(path, token)
      .then((data) => {
        if (!active) return;
        setPosts(data.posts);
        setMeta(data.meta);
        setError("");
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
  }, [token, path]);

  if (loading || !session || !ready) return <FeedLoadingFrame />;

  const hasNext = meta.page * meta.per_page < meta.total;

  return (
    <>
      {error ? <ErrorState message={error} /> : null}
      {!error && posts.length === 0 ? (
        <EmptyState
          title="まだ保存がありません"
          body="投稿の保存ボタンから残せます。"
        />
      ) : null}
      {posts.length > 0 ? (
        <FeedGrid>
          {posts.map((post) => (
            <FeedCard key={post.id} post={post} />
          ))}
        </FeedGrid>
      ) : null}
      {hasNext ? (
        <Link href={`/me/saved?page=${meta.page + 1}`} className="block w-full rounded-full border border-white/20 glass bg-white/10 py-3 text-center text-sm font-semibold text-slate-200 shadow-xs transition hover:bg-white/16 hover:text-white active:scale-[0.98]">
          次の投稿
        </Link>
      ) : null}
    </>
  );
}
