"use client";

import Link from "next/link";
import { useEffect, useState } from "react";
import type { PostsPage } from "@/lib/fetch-posts";
import { homeFeedPath, isFeedFresh, loadHomeFeed, prefetchHomeFeeds } from "@/lib/home-feed-cache";
import { FeedCard, feedGridClass } from "@/components/feed-card";
import { FeedLoadingFrame } from "@/components/loading-frame";
import { EmptyState, ErrorState } from "@/components/empty-state";
import type { Meta, Post } from "@/lib/types";

const loadError = "投稿の取得に失敗しました。APIの起動を確認してください。";

function nextHref(sort: "popular" | "new", q: string, page: number, kind?: "recruitment") {
  const params = new URLSearchParams();
  if (kind === "recruitment") params.set("sort", "recruitment");
  else if (sort !== "popular") params.set("sort", sort);
  if (q) params.set("q", q);
  params.set("page", String(page));
  return `/?${params}`;
}

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
  const [posts, setPosts] = useState<Post[] | null>(initial?.posts ?? null);
  const [meta, setMeta] = useState<Meta>(initial?.meta ?? { page, per_page: 20, total: 0 });
  const [error, setError] = useState("");
  const hasInitial = initial !== null;
  const path = homeFeedPath(kind ?? sort, q, page);

  useEffect(() => {
    let active = true;
    if (hasInitial && isFeedFresh(path)) {
      prefetchHomeFeeds(q);
      return;
    }
    // Without data, keep retrying while the API wakes up; with stale data, refresh quietly.
    loadHomeFeed(path, !hasInitial)
      .then((data) => {
        if (!active) return;
        setPosts(data.posts);
        setMeta(data.meta);
        setError("");
        prefetchHomeFeeds(q);
      })
      .catch(() => {
        if (active && !hasInitial) setError(loadError);
      });
    return () => {
      active = false;
    };
  }, [hasInitial, path, q]);

  if (error) return <ErrorState message={error} />;
  if (posts === null) return <FeedLoadingFrame />;

  const hasNext = meta.page * meta.per_page < meta.total;

  return (
    <>
      {posts.length === 0 ? (
        <EmptyState
          title={q ? "一致する投稿がありません" : kind === "recruitment" ? "まだ募集がありません" : "まだ投稿がありません"}
          body={
            q
              ? "別の言葉で探してください。"
              : kind === "recruitment"
                ? "クラブチームの募集と練習相手募集から探せます。"
                : "最初のレース感想を書いてみましょう。"
          }
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
        <Link
          href={nextHref(sort, q, meta.page + 1, kind)}
          className="block w-full rounded-md border bg-white py-3 text-center text-sm font-medium"
        >
          次の投稿
        </Link>
      ) : null}
    </>
  );
}
