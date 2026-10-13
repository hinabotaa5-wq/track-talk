"use client";

import { useEffect, useState } from "react";
import { useSearchParams } from "next/navigation";
import { useAuth } from "@/components/auth-provider";
import { HomeFeed } from "@/components/home-feed";
import { SavedFeed } from "@/components/saved-feed";
import type { PostsPage } from "@/lib/fetch-posts";
import { homeFeedPath, prefetchSavedFeed, readFeed, seedFeed, savedFeedPath } from "@/lib/home-feed-cache";
import { parseHomeSort } from "@/lib/home-sort";

// The filter chips change the URL with history.pushState, so this reads the tab from the URL
// instead of server props and switches without a server round trip.
export function HomeView({ initialPath, initial }: { initialPath: string; initial: PostsPage | null }) {
  const sp = useSearchParams();
  const sort = parseHomeSort(sp.get("sort"));
  const q = (sp.get("q") ?? "").trim();
  const page = Math.max(1, Number(sp.get("page")) || 1);
  const { token } = useAuth();

  useState(() => {
    if (initial && typeof window !== "undefined") seedFeed(initialPath, initial);
  });

  useEffect(() => {
    if (token) prefetchSavedFeed(q, token);
  }, [token, q]);

  if (sort === "saved") {
    return <SavedFeed key={savedFeedPath(q, page)} q={q} page={page} />;
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
