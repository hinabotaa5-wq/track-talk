import type { Metadata } from "next";
import { Suspense } from "react";
import { apiFetch } from "@/lib/api";
import { HomeView } from "@/components/home-view";
import { FeedLoadingFrame } from "@/components/loading-frame";
import { TodayMeetsSection } from "@/components/today-meets-section";
import { homeFeedServerTimeoutMs, type PostsPage } from "@/lib/fetch-posts";
import { homeFeedPath } from "@/lib/home-feed-cache";
import { parseHomeSort } from "@/lib/home-sort";
import { fetchTodayMeetThreads } from "@/lib/meet-threads";
import type { Meta, Post } from "@/lib/types";

export const dynamic = "force-dynamic";

type HomeSearchParams = Promise<{ sort?: string; q?: string; page?: string }>;

// 並び替えやページ送りは同じ一覧の別表示なので "/" にまとめ、検索結果のページは検索エンジンに載せない。
export async function generateMetadata({ searchParams }: { searchParams: HomeSearchParams }): Promise<Metadata> {
  const sp = await searchParams;
  return {
    alternates: { canonical: "/" },
    ...((sp.q ?? "").trim() ? { robots: { index: false, follow: true } } : {}),
  };
}

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
  searchParams: HomeSearchParams;
}) {
  const sp = await searchParams;
  const sort = parseHomeSort(sp.sort);
  const q = (sp.q ?? "").trim();
  const page = Math.max(1, Number(sp.page) || 1);

  if (sort === "review") return <HomeView initialPath="" initial={null} />;

  const path = homeFeedPath(sort, q, page);
  // 検索結果や2ページ目には出さない。対象大会が無い日は空配列になり、セクションごと消える。
  const meets = q || page > 1 ? [] : await fetchTodayMeetThreads();
  return (
    <>
      <TodayMeetsSection meets={meets} />
      <Suspense key={path} fallback={<FeedLoadingFrame />}>
        <ServerHomeFeed path={path} />
      </Suspense>
    </>
  );
}
