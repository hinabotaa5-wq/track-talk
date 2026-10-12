import { Suspense } from "react";
import { apiFetch } from "@/lib/api";
import { HomeFeed } from "@/components/home-feed";
import { FeedLoadingFrame } from "@/components/loading-frame";
import { SavedFeed } from "@/components/saved-feed";
import { homeFeedServerTimeoutMs, type PostsPage } from "@/lib/fetch-posts";
import { parseHomeSort } from "@/lib/home-sort";
import type { Meta, Post } from "@/lib/types";

export const dynamic = "force-dynamic";

async function ServerHomeFeed({
  sort,
  kind,
  q,
  page,
}: {
  sort: "popular" | "new";
  kind?: "recruitment";
  q: string;
  page: number;
}) {
  const qs = new URLSearchParams({ sort, page: String(page), per_page: "20" });
  if (q) qs.set("q", q);
  if (kind) qs.set("kind", kind);

  // On timeout or failure, HomeFeed retries from the browser when initial is null.
  let initial: PostsPage | null = null;
  try {
    const data = await apiFetch<{ posts: Post[]; meta: Meta }>(`/v1/posts?${qs}`, {
      signal: AbortSignal.timeout(homeFeedServerTimeoutMs),
    });
    initial = { posts: data.posts ?? [], meta: data.meta };
  } catch {
    initial = null;
  }

  return <HomeFeed sort={sort} kind={kind} q={q} page={page} initial={initial} />;
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

  if (sort === "saved") return <SavedFeed q={q} page={page} />;

  return (
    <Suspense key={`${sort}:${q}:${page}`} fallback={<FeedLoadingFrame />}>
      <ServerHomeFeed
        sort={sort === "recruitment" ? "new" : sort}
        kind={sort === "recruitment" ? "recruitment" : undefined}
        q={q}
        page={page}
      />
    </Suspense>
  );
}
