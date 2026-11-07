"use client";

import Link from "next/link";
import { useState } from "react";
import { useSearchParams } from "next/navigation";
import dynamic from "next/dynamic";
import { useAuth } from "@/components/auth-provider";
import { PushPermissionGuide } from "@/components/push-permission-guide";
import { HomeFeed } from "@/components/home-feed";
import { FeedLoadingFrame } from "@/components/loading-frame";
import type { PostsPage } from "@/lib/fetch-posts";
import { homeFeedPath, isRenderFresh, readFeed, renderSeedTime, seedFeed } from "@/lib/home-feed-cache";
import { parseHomeSort, parseProductKind } from "@/lib/home-sort";
import { profileGateRedirecting } from "@/lib/profile-gate";
import { parseReviewFilters, reviewFiltersQuery } from "@/lib/review-filters";

// レビューとそのアニメーションは、レビューを開くときだけ読み込む。
const ReviewHome = dynamic(() => import("@/components/review-home").then((mod) => mod.ReviewHome), {
  loading: () => <FeedLoadingFrame horizontal />,
});

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
  const { loading, profile, session, token } = useAuth();
  const userId = session?.user?.id;
  const sort = parseHomeSort(sp.get("sort"));
  const q = (sp.get("q") ?? "").trim();
  const page = Math.max(1, Number(sp.get("page")) || 1);

  useState(() => {
    if (initial && typeof window !== "undefined") seedFeed(initialPath, initial, renderSeedTime(renderedAt));
  });

  const pushGuide = !loading && userId && token && !profileGateRedirecting("/", profile) ? (
    <PushPermissionGuide key={userId} userId={userId} token={token} home />
  ) : null;

  if (sort === "review") {
    const kind = parseProductKind(sp.get("type"));
    const filters = parseReviewFilters(sp);
    return (
      <>
        <ReviewHome key={`${kind}:${q}:${page}:${reviewFiltersQuery(filters)}`} kind={kind} q={q} page={page} filters={filters} />
        {pushGuide}
      </>
    );
  }

  const path = homeFeedPath(sort, q, page);
  const data = readFeed(path) ?? (path === initialPath ? initial : null);
  const feed = (
    <HomeFeed
      key={path}
      sort={sort === "recruitment" ? "new" : sort}
      kind={sort === "recruitment" ? "recruitment" : undefined}
      q={q}
      page={page}
      initial={data}
    />
  );
  // 募集タブからは、練習相手の募集を都道府県と種目で探せる一覧へ行ける。
  return (
    <>
      {sort === "recruitment" ? (
        <Link href="/practice" className="mb-3 inline-flex min-h-11 items-center justify-self-start text-sm font-medium text-sky-200 underline underline-offset-4">
          練習相手を都道府県と種目で探す
        </Link>
      ) : null}
      {feed}
      {pushGuide}
    </>
  );
}
