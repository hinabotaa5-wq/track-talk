"use client";

import Link from "next/link";
import { useRouter, useSearchParams } from "next/navigation";
import { useEffect, useState } from "react";
import { EmptyState, ErrorState } from "@/components/empty-state";
import { FeedCard } from "@/components/feed-card";
import { FeedGrid } from "@/components/feed-grid";
import { LoadMore } from "@/components/load-more";
import { PageLoading } from "@/components/page-loading";
import { apiFetch, ApiRequestError } from "@/lib/api";
import { appEvents } from "@/lib/practice-events";
import {
  practiceCountLabel,
  practiceFilterFrom,
  practiceListPath,
  practicePagePath,
  type PracticeFilter,
  type PracticeList,
} from "@/lib/practice-list";
import { PREFECTURES } from "@/lib/prefectures";

const selectClass =
  "mt-1 w-full rounded-md border border-white/25 glass bg-white/10 px-3 py-2 text-base text-slate-100 outline-none focus:border-blue-400";

type State = { key: string; list: PracticeList | null; error: string; moreError: string; expired: boolean };

// 練習相手の一覧。都道府県と種目で絞れ、どちらも最初は「すべて」。ログインしていなくても見られる。
export function PracticeListView() {
  const router = useRouter();
  const params = useSearchParams();
  const filter = practiceFilterFrom(params.get("prefecture"), params.get("event"));
  const key = practiceListPath(filter);
  const [state, setState] = useState<State | null>(null);
  const [loadingMore, setLoadingMore] = useState(false);
  const current = state?.key === key ? state : null;

  useEffect(() => {
    const controller = new AbortController();
    apiFetch<PracticeList>(key, { signal: controller.signal })
      .then((list) => {
        if (!controller.signal.aborted) setState({ key, list, error: "", moreError: "", expired: false });
      })
      .catch(() => {
        if (!controller.signal.aborted) setState({ key, list: null, error: "募集の取得に失敗しました。時間をおいて開き直してください。", moreError: "", expired: false });
      });
    return () => controller.abort();
  }, [key]);

  function change(next: Partial<PracticeFilter>) {
    router.replace(practicePagePath({ ...filter, ...next }), { scroll: false });
  }

  async function loadMore() {
    const list = current?.list;
    if (!list?.next_cursor || loadingMore) return;
    setLoadingMore(true);
    try {
      const more = await apiFetch<PracticeList>(practiceListPath(filter, list.next_cursor));
      setState((value) => {
        if (!value || value.key !== key || !value.list) return value;
        const seen = new Set(value.list.posts.map((post) => post.id));
        const added = more.posts.filter((post) => !seen.has(post.id));
        return { ...value, list: { posts: [...value.list.posts, ...added], total: more.total, next_cursor: added.length ? more.next_cursor : "" }, moreError: "" };
      });
    } catch (error) {
      const expired = error instanceof ApiRequestError && error.code === "invalid_cursor";
      setState((value) => (value && value.key === key ? { ...value, moreError: "続きの読み込みに失敗しました", expired } : value));
    } finally {
      setLoadingMore(false);
    }
  }

  return (
    <div className="mx-auto max-w-2xl space-y-4">
      <div>
        <h1 className="text-2xl font-bold text-slate-100">練習相手募集</h1>
        <p className="mt-1 text-sm text-slate-200">これから開かれる練習の募集です。</p>
      </div>
      <div className="grid grid-cols-2 gap-3">
        <label className="block text-sm font-medium">
          都道府県
          <select className={selectClass} value={filter.prefecture} onChange={(e) => change({ prefecture: e.target.value })}>
            <option value="">すべて</option>
            {PREFECTURES.map((item) => (
              <option key={item.code} value={item.code}>
                {item.name}
              </option>
            ))}
          </select>
        </label>
        <label className="block text-sm font-medium">
          種目
          <select className={selectClass} value={filter.event} onChange={(e) => change({ event: e.target.value })}>
            <option value="">すべて</option>
            {appEvents.map((item) => (
              <option key={item.value} value={item.value}>
                {item.label}
              </option>
            ))}
          </select>
        </label>
      </div>
      {current?.error ? (
        <ErrorState message={current.error} />
      ) : !current?.list ? (
        <PageLoading label="読み込み中..." />
      ) : (
        <>
          <p className="text-sm font-semibold text-slate-100">{practiceCountLabel(filter, current.list.total)}</p>
          {current.list.posts.length === 0 ? (
            <EmptyState title="これからの募集はまだありません" body="条件を変えるか、自分で募集してみましょう。" />
          ) : (
            <FeedGrid>
              {current.list.posts.map((post) => (
                <FeedCard key={post.id} post={post} />
              ))}
            </FeedGrid>
          )}
          {current.list.next_cursor ? (
            <LoadMore onLoad={() => void loadMore()} loading={loadingMore} error={current.moreError} expired={current.expired} />
          ) : null}
        </>
      )}
      <Link href="/posts/practice" className="inline-flex min-h-11 items-center text-sm font-medium text-sky-200 underline underline-offset-4">
        練習相手を募集する
      </Link>
    </div>
  );
}
