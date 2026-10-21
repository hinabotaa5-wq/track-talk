"use client";

import { useState } from "react";
import { useSearchParams } from "next/navigation";
import { HomeFeed } from "@/components/home-feed";
import { ReviewHome } from "@/components/review-home";
import type { PostsPage } from "@/lib/fetch-posts";
import { homeFeedPath, isRenderFresh, readFeed, renderSeedTime, seedFeed } from "@/lib/home-feed-cache";
import { parseHomeSort, parseProductKind } from "@/lib/home-sort";
import { parseReviewFilters, reviewFiltersQuery } from "@/lib/review-filters";

// 絞り込みのチップは history.pushState で URL を変えるので、タブはサーバーの props ではなく URL から読み、
// サーバーとやりとりせずに切り替える。
export function HomeView({
  initialPath,
  initial: rendered,
  renderedAt,
}: {
  initialPath: string;
  initial: PostsPage | null;
  renderedAt: number;
}) {
  // 投稿を作ったり消したりしたあとに、前に描いた画面の古い一覧が出ないようにする。
  const initial = isRenderFresh(renderedAt) ? rendered : null;
  const sp = useSearchParams();
  const sort = parseHomeSort(sp.get("sort"));
  const q = (sp.get("q") ?? "").trim();
  const page = Math.max(1, Number(sp.get("page")) || 1);

  useState(() => {
    if (initial && typeof window !== "undefined") seedFeed(initialPath, initial, renderSeedTime(renderedAt));
  });

  if (sort === "review") {
    const kind = parseProductKind(sp.get("type"));
    const filters = parseReviewFilters(sp);
    return <ReviewHome key={`${kind}:${q}:${page}:${reviewFiltersQuery(filters)}`} kind={kind} q={q} page={page} filters={filters} />;
  }

  const path = homeFeedPath(sort, q, page);
  const data = readFeed(path) ?? (path === initialPath ? initial : null);
  return (
    <HomeFeed
      key={path}
      sort={sort === "recruitment" ? "new" : sort}
      kind={sort === "recruitment" ? "recruitment" : undefined}
      q={q}
      page={page}
      initial={data}
    />
  );
}
