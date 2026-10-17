"use client";

import { useState } from "react";
import { useSearchParams } from "next/navigation";
import { HomeFeed } from "@/components/home-feed";
import { ReviewHome } from "@/components/review-home";
import type { PostsPage } from "@/lib/fetch-posts";
import { homeFeedPath, readFeed, seedFeed } from "@/lib/home-feed-cache";
import { parseHomeSort, parseProductKind } from "@/lib/home-sort";
import { parseReviewFilters, reviewFiltersQuery } from "@/lib/review-filters";

// The filter chips change the URL with history.pushState, so this reads the tab from the URL
// instead of server props and switches without a server round trip.
export function HomeView({ initialPath, initial }: { initialPath: string; initial: PostsPage | null }) {
  const sp = useSearchParams();
  const sort = parseHomeSort(sp.get("sort"));
  const q = (sp.get("q") ?? "").trim();
  const page = Math.max(1, Number(sp.get("page")) || 1);

  useState(() => {
    if (initial && typeof window !== "undefined") seedFeed(initialPath, initial);
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
