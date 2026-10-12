"use client";

import Link from "next/link";
import { useEffect, useState } from "react";
import { fetchPostsRetrying, type PostsPage } from "@/lib/fetch-posts";
import { FeedCard, feedGridClass } from "@/components/feed-card";
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
  const shouldRetry = initial === null;

  useEffect(() => {
    if (!shouldRetry) return;
    const controller = new AbortController();
    const qs = new URLSearchParams({ sort, page: String(page), per_page: "20" });
    if (q) qs.set("q", q);
    if (kind) qs.set("kind", kind);
    fetchPostsRetrying(`/v1/posts?${qs}`, { signal: controller.signal })
      .then((data) => {
        if (controller.signal.aborted) return;
        setPosts(data.posts);
        setMeta(data.meta);
      })
      .catch(() => {
        if (controller.signal.aborted) return;
        setError(loadError);
      });
    return () => controller.abort();
  }, [shouldRetry, sort, q, page, kind]);

  if (error) return <ErrorState message={error} />;
  if (posts === null) return <p role="status">読み込み中…</p>;

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
