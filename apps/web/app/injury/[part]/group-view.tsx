"use client";

import Link from "next/link";
import { useEffect, useState } from "react";
import { FeedCard, feedGridClass } from "@/components/feed-card";
import { FeedLoadingFrame } from "@/components/loading-frame";
import { EmptyState, ErrorState } from "@/components/empty-state";
import { InjuryJoinWizard } from "@/components/injury-join-wizard";
import { useAuth } from "@/components/auth-provider";
import { apiFetch, ApiRequestError } from "@/lib/api";
import type { PostsPage } from "@/lib/fetch-posts";
import { injuryBodyPartLabel, injuryNotice, type InjuryGroup } from "@/lib/injury";

type Load = { kind: "loading" } | { kind: "joined"; group: InjuryGroup } | { kind: "not-joined" } | { kind: "failed" };

export function GroupView({ part }: { part: string }) {
  const { session, token, loading } = useAuth();
  const [load, setLoad] = useState<Load>({ kind: "loading" });
  const [posts, setPosts] = useState<PostsPage | null>(null);

  useEffect(() => {
    if (loading || !token) return;
    const controller = new AbortController();
    apiFetch<InjuryGroup>(`/v1/injury/groups/${part}`, { token, signal: controller.signal })
      .then((group) => setLoad({ kind: "joined", group }))
      .catch((e) => {
        if (controller.signal.aborted) return;
        setLoad(e instanceof ApiRequestError && e.code === "not_joined" ? { kind: "not-joined" } : { kind: "failed" });
      });
    return () => controller.abort();
  }, [part, token, loading]);

  const joined = load.kind === "joined";
  useEffect(() => {
    if (!joined || !token) return;
    const controller = new AbortController();
    apiFetch<PostsPage>(`/v1/injury/groups/${part}/posts?per_page=20`, { token, signal: controller.signal })
      .then((page) => setPosts({ posts: page.posts ?? [], meta: page.meta }))
      .catch(() => {
        if (!controller.signal.aborted) setPosts({ posts: [], meta: { page: 1, per_page: 20, total: 0 } });
      });
    return () => controller.abort();
  }, [joined, part, token]);

  const back = (
    <Link href="/injury" className="inline-block text-sm font-semibold text-blue-300 underline-offset-2 hover:underline">
      ← ケガ人の溜まり場
    </Link>
  );

  if (!loading && !session) {
    return (
      <div className="space-y-4">
        {back}
        <EmptyState title="ログインすると入れます" body="グループの中は、参加した人だけが読めます。" />
        <Link href="/login" className="block rounded-full bg-blue-600 py-3 text-center text-sm font-semibold text-white">
          ログイン
        </Link>
      </div>
    );
  }

  if (load.kind === "failed") {
    return (
      <div className="space-y-4">
        {back}
        <ErrorState message="グループの取得に失敗しました。APIの起動を確認してください。" />
      </div>
    );
  }

  if (load.kind === "not-joined") {
    return (
      <div className="space-y-4">
        {back}
        <InjuryJoinWizard
          part={part}
          token={token}
          onJoined={() => {
            setLoad({ kind: "loading" });
            setPosts(null);
            apiFetch<InjuryGroup>(`/v1/injury/groups/${part}`, { token })
              .then((group) => setLoad({ kind: "joined", group }))
              .catch(() => setLoad({ kind: "failed" }));
          }}
        />
      </div>
    );
  }

  if (load.kind === "loading") {
    return (
      <div className="space-y-4">
        {back}
        <FeedLoadingFrame />
      </div>
    );
  }

  const { group } = load;
  return (
    <div className="space-y-4">
      {back}
      <div>
        <h1 className="text-2xl font-bold">{injuryBodyPartLabel(group.body_part)}の仲間 · {group.members}人</h1>
        <p className="mt-1 text-sm text-slate-200">いまの段階: {group.me.stage_label}</p>
      </div>
      {posts === null ? (
        <FeedLoadingFrame />
      ) : posts.posts.length === 0 ? (
        <EmptyState title="まだ投稿がありません" body="最初の一言を書いてみましょう。" />
      ) : (
        <div className={feedGridClass}>
          {posts.posts.map((post) => (
            <FeedCard key={post.id} post={post} />
          ))}
        </div>
      )}
      <p className="text-xs text-slate-300">{injuryNotice}</p>
    </div>
  );
}
