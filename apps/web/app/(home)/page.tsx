import { Suspense } from "react";
import { apiFetch } from "@/lib/api";
import { HomeView } from "@/components/home-view";
import { FeedLoadingFrame } from "@/components/loading-frame";
import { homeFeedServerTimeoutMs, type PostsPage } from "@/lib/fetch-posts";
import { homeFeedPath } from "@/lib/home-feed-cache";
import { parseHomeSort } from "@/lib/home-sort";
import type { Meta, Post } from "@/lib/types";

export const dynamic = "force-dynamic";

async function ServerHomeFeed({ path }: { path: string }) {
  // On timeout or failure, HomeFeed retries from the browser when initial is null.
  let initial: PostsPage | null = null;
  try {
    const data = await apiFetch<{ posts: Post[]; meta: Meta }>(path, {
      signal: AbortSignal.timeout(homeFeedServerTimeoutMs),
    });
    initial = { posts: data.posts ?? [], meta: data.meta };
  } catch {
    initial = null;
  }

  return <HomeView initialPath={path} initial={initial} />;
}

export default async function HomePage({
  searchParams,
}: {
  searchParams: Promise<{ sort?: string; q?: string; page?: string }>;
}) {
  const sp = await searchParams;
  const sort = parseHomeSort(sp.sort);
  const q = (sp.q ?? "").trim();
  const page = Math.max(1, Number(sp.page) || 1);

  if (sort === "saved" || sort === "review") return <HomeView initialPath="" initial={null} />;

  const path = homeFeedPath(sort, q, page);
  return (
    <Suspense key={path} fallback={<FeedLoadingFrame />}>
      <ServerHomeFeed path={path} />
    </Suspense>
  );
}
