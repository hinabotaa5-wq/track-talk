"use client";

import Link from "next/link";
import { useEffect, useState } from "react";
import { usePathname, useRouter, useSearchParams } from "next/navigation";
import { FeedCard, feedGridClass } from "@/components/feed-card";
import { FeedLoadingFrame } from "@/components/loading-frame";
import { EmptyState, ErrorState } from "@/components/empty-state";
import { useAuth } from "@/components/auth-provider";
import { fetchPostsRetrying, type PostsPage } from "@/lib/fetch-posts";
import {
  injuryBodyParts,
  injuryFeedPath,
  injuryNotice,
  injuryStages,
  isInjuryBodyPart,
  isInjuryStage,
} from "@/lib/injury";

export function InjuryView() {
  const sp = useSearchParams();
  const router = useRouter();
  const pathname = usePathname();
  const { session } = useAuth();
  const partParam = sp.get("part");
  const part = isInjuryBodyPart(partParam) ? (partParam ?? "") : "";
  const stageParam = sp.get("stage");
  const stage = isInjuryStage(stageParam) ? stageParam : "";
  const page = Math.max(1, Number(sp.get("page")) || 1);
  const path = injuryFeedPath(part, page, stage);

  const [data, setData] = useState<{ path: string; page: PostsPage } | null>(null);
  const [failed, setFailed] = useState("");

  useEffect(() => {
    const controller = new AbortController();
    fetchPostsRetrying(path, { signal: controller.signal })
      .then((result) => {
        setData({ path, page: result });
        setFailed("");
      })
      .catch(() => {
        if (!controller.signal.aborted) setFailed(path);
      });
    return () => controller.abort();
  }, [path]);

  function go(next: { part?: string; stage?: string; page?: number }) {
    const qs = new URLSearchParams();
    const nextPart = next.part ?? part;
    if (nextPart) qs.set("part", nextPart);
    const nextStage = next.stage ?? stage;
    if (nextStage) qs.set("stage", nextStage);
    if (next.page && next.page > 1) qs.set("page", String(next.page));
    const query = qs.toString();
    router.replace(query ? `${pathname}?${query}` : pathname, { scroll: false });
  }

  const current = data?.path === path ? data.page : null;
  const hasNext = current ? current.meta.page * current.meta.per_page < current.meta.total : false;
  const filtered = part !== "" || stage !== "";

  return (
    <div className="space-y-4">
      <div className="flex items-start justify-between gap-3">
        <div>
          <h1 className="text-2xl font-bold">ケガ人の溜まり場</h1>
          <p className="mt-1 text-sm text-slate-200">
            走れないつらさも、いまできる頑張りも、ここで分け合おう。
          </p>
        </div>
        <Link
          href={session ? "/injury/new" : "/login"}
          className="shrink-0 rounded-full bg-blue-600 px-4.5 py-2 text-sm font-semibold text-white shadow-xs transition hover:bg-blue-500 active:scale-95"
        >
          投稿する
        </Link>
      </div>

      <Link href="/injury/stats" className="inline-block text-sm font-semibold text-blue-300 underline-offset-2 hover:underline">
        みんなの復帰データを見る →
      </Link>

      <div className="flex flex-wrap items-center gap-2" role="group" aria-label="絞り込み">
        <select
          aria-label="部位で絞り込む"
          className="rounded-full border border-white/25 glass bg-white/10 px-3 py-1.5 text-sm text-slate-100 outline-none focus:border-blue-400"
          value={part}
          onChange={(e) => go({ part: e.target.value, page: 1 })}
        >
          <option value="">すべての部位</option>
          {injuryBodyParts.map((item) => (
            <option key={item.value} value={item.value}>
              {item.label}
            </option>
          ))}
        </select>
        <select
          aria-label="復帰の段階で絞り込む"
          className="rounded-full border border-white/25 glass bg-white/10 px-3 py-1.5 text-sm text-slate-100 outline-none focus:border-blue-400"
          value={stage}
          onChange={(e) => go({ stage: e.target.value, page: 1 })}
        >
          <option value="">すべての段階</option>
          {injuryStages.map((item) => (
            <option key={item.value} value={item.value}>
              {item.label}
            </option>
          ))}
        </select>
      </div>

      {failed === path ? (
        <ErrorState message="投稿の取得に失敗しました。APIの起動を確認してください。" />
      ) : current === null ? (
        <FeedLoadingFrame />
      ) : current.posts.length === 0 ? (
        <EmptyState
          title={filtered ? "一致する投稿がありません" : "まだ投稿がありません"}
          body={filtered ? "条件を変えて探してみてください。" : "最初の一言を書いてみましょう。"}
        />
      ) : (
        <div className={feedGridClass}>
          {current.posts.map((post) => (
            <FeedCard key={post.id} post={post} />
          ))}
        </div>
      )}

      {hasNext ? (
        <button
          type="button"
          onClick={() => go({ page: page + 1 })}
          className="block w-full rounded-full border border-white/20 glass bg-white/10 py-3 text-center text-sm font-semibold text-slate-200 shadow-xs transition hover:bg-white/16 hover:text-white active:scale-95"
        >
          次の投稿
        </button>
      ) : null}

      <p className="text-xs text-slate-300">{injuryNotice}</p>
    </div>
  );
}
