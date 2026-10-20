"use client";

import Link from "next/link";
import { useRouter } from "next/navigation";
import { useCallback, useEffect, useState, type ReactNode } from "react";
import { FeedCard, feedGridClass } from "@/components/feed-card";
import { FeedLoadingFrame } from "@/components/loading-frame";
import { EmptyState, ErrorState } from "@/components/empty-state";
import { InjuryGoalCelebration } from "@/components/injury-goal-celebration";
import { InjuryJoinWizard } from "@/components/injury-join-wizard";
import { InjuryRoadmap } from "@/components/injury-roadmap";
import { InjuryStageChange } from "@/components/injury-stage-change";
import { Button } from "@/components/ui/button";
import { useAuth } from "@/components/auth-provider";
import { apiFetch, ApiRequestError } from "@/lib/api";
import { fetchAllPages } from "@/lib/paging";
import type { PostsPage } from "@/lib/fetch-posts";
import {
  canPostInStage,
  injuryBodyPartLabel,
  injuryGroupDescriptions,
  injuryGroupTitle,
  injuryStages,
  type InjuryGroup,
  type InjuryGroupMember,
  type InjuryStage,
} from "@/lib/injury";

type Load = { kind: "loading" } | { kind: "joined"; group: InjuryGroup } | { kind: "not-joined" } | { kind: "failed" };

export function GroupView({ part, members, summary }: { part: string; members: number | null; summary: ReactNode }) {
  const { session, token, loading } = useAuth();
  const [load, setLoad] = useState<Load>({ kind: "loading" });
  const [selected, setSelected] = useState<InjuryStage | null>(null);
  const [posts, setPosts] = useState<{ stage: string; page: PostsPage } | null>(null);
  const router = useRouter();
  const [changing, setChanging] = useState(false);
  const [leaving, setLeaving] = useState<"idle" | "confirm" | "busy">("idle");
  const [leaveError, setLeaveError] = useState("");
  const [celebrate, setCelebrate] = useState(false);
  const [joining, setJoining] = useState(false);

  const applyGroup = useCallback((group: InjuryGroup) => {
    setLoad({ kind: "joined", group });
    setSelected((current) => current ?? group.me.stage);
  }, []);

  const applyGroupError = useCallback((e: unknown) => {
    setLoad(e instanceof ApiRequestError && e.code === "not_joined" ? { kind: "not-joined" } : { kind: "failed" });
  }, []);

  const fetchGroup = useCallback(
    () => apiFetch<InjuryGroup>(`/v1/injury/groups/${part}`, { token }).then(applyGroup).catch(applyGroupError),
    [part, token, applyGroup, applyGroupError],
  );

  useEffect(() => {
    if (loading || !token) return;
    const controller = new AbortController();
    apiFetch<InjuryGroup>(`/v1/injury/groups/${part}`, { token, signal: controller.signal })
      .then(applyGroup)
      .catch((e) => {
        if (!controller.signal.aborted) applyGroupError(e);
      });
    return () => controller.abort();
  }, [loading, token, part, applyGroup, applyGroupError]);

  const joined = load.kind === "joined";
  const myStage = joined ? load.group.me.stage : null;
  useEffect(() => {
    if (!joined || !token || !selected) return;
    const controller = new AbortController();
    // 21件目以降もグループの中から辿れるよう、最後のページまで集める。
    fetchAllPages(async (pageNo) => {
      const page = await apiFetch<PostsPage>(`/v1/injury/groups/${part}/posts?page=${pageNo}&per_page=50&stage=${selected}`, {
        token,
        signal: controller.signal,
      });
      return { items: page.posts ?? [], total: page.meta.total };
    })
      .then((all) => setPosts({ stage: selected, page: { posts: all.items, meta: { page: 1, per_page: all.items.length, total: all.total } } }))
      .catch(() => {
        if (!controller.signal.aborted) setPosts({ stage: selected, page: { posts: [], meta: { page: 1, per_page: 20, total: 0 } } });
      });
    return () => controller.abort();
  }, [joined, part, token, selected, myStage]);

  async function onStageChanged(member: InjuryGroupMember, celebrate: boolean) {
    setChanging(false);
    setSelected(member.stage);
    setPosts(null);
    if (celebrate) setCelebrate(true);
    await fetchGroup();
  }

  async function leaveGroup() {
    setLeaving("busy");
    setLeaveError("");
    try {
      await apiFetch(`/v1/injury/groups/${part}/me`, { method: "DELETE", token });
      router.push("/injury");
    } catch (e) {
      setLeaveError(e instanceof ApiRequestError ? e.message : "グループを抜けられませんでした");
      setLeaving("confirm");
    }
  }

  // 入っていない人(ログインしていない人や検索から来た人も)には、グループの紹介と入るボタンだけを見せる。
  // サーバーで描くときはまだログインがわからないので、最初はこの形で描く。
  const intro = (cta: ReactNode) => (
    <div className="space-y-4">
      <div>
        <h1 className="text-2xl font-bold">
          {injuryGroupTitle(part)}
          {members !== null ? <span className="text-lg font-semibold text-slate-200"> · {members}人</span> : null}
        </h1>
        <p className="mt-2 text-sm text-slate-200">{injuryGroupDescriptions[part]}</p>
      </div>
      {cta}
    </div>
  );
  const ctaClass = "block w-full rounded-full bg-blue-600 py-3 text-center text-sm font-semibold text-white transition hover:bg-blue-500 active:scale-95";

  if (!loading && !session) {
    return intro(
      <Link href={`/login?next=${encodeURIComponent(`/injury/${part}`)}`} className={ctaClass}>
        ログインしてグループに入る
      </Link>,
    );
  }

  if (load.kind === "failed") {
    return intro(<ErrorState message="グループの取得に失敗しました。APIの起動を確認してください。" />);
  }

  if (load.kind === "not-joined") {
    if (!joining) {
      return intro(
        <button type="button" onClick={() => setJoining(true)} className={ctaClass}>
          このグループに入る
        </button>,
      );
    }
    return (
      <div className="space-y-4">
          <InjuryJoinWizard
          part={part}
          token={token}
          onJoined={() => {
            setLoad({ kind: "loading" });
            setSelected(null);
            setPosts(null);
            setJoining(false);
            void fetchGroup();
          }}
        />
      </div>
    );
  }

  if (load.kind === "loading" || !selected) {
    return intro(<div className="h-11" aria-hidden />);
  }

  const { group } = load;
  const mine = group.me.stage;
  const own = canPostInStage(selected, mine);
  const selectedLabel = injuryStages.find((item) => item.value === selected)?.label ?? "";
  const current = posts?.stage === selected ? posts.page : null;

  return (
    <div className="space-y-4">
      <div className="flex items-start justify-between gap-3">
        <div>
          <h1 className="text-2xl font-bold">
            {injuryGroupTitle(part)} · {group.members}人
          </h1>
          <p className="mt-1 text-sm text-slate-200">段階をタップで読めます</p>
        </div>
        <Button variant="secondary" className="shrink-0" onClick={() => setChanging((v) => !v)} aria-expanded={changing}>
          段階を変える
        </Button>
      </div>

      {celebrate ? <InjuryGoalCelebration onClose={() => setCelebrate(false)} /> : null}

      {changing ? (
        <InjuryStageChange
          part={part}
          token={token}
          member={group.me}
          onChanged={(member, celebrate) => void onStageChanged(member, celebrate)}
          onCancel={() => setChanging(false)}
        />
      ) : null}

      <InjuryRoadmap stages={group.stages} mine={mine} selected={selected} onSelect={setSelected} />

      <h2 className="pt-2 text-base font-bold">{selectedLabel}の投稿</h2>
      {current === null ? (
        <FeedLoadingFrame />
      ) : current.posts.length === 0 ? (
        <EmptyState title="まだ投稿がありません" body={own ? "最初の一言を書いてみましょう。" : "この段階の投稿は、まだありません。"} />
      ) : (
        <div className={feedGridClass}>
          {current.posts.map((post) => (
            <FeedCard key={post.id} post={post} />
          ))}
        </div>
      )}

      {summary ? (
        <section className="space-y-3 border-t border-white/15 pt-4" aria-label="みんなのデータ">
          <div>
            <h2 className="text-base font-bold">みんなのデータ</h2>
            <p className="mt-1 text-xs text-slate-300">グループ外の人も見られます</p>
          </div>
          {summary}
        </section>
      ) : null}

      {/* 投稿ボタンはスクロールしているあいだ下のメニューのすぐ上に貼りつき、いちばん下まで来ると
          「このグループを抜ける」の上に収まる。段階を変えるフォームを書いているあいだは、入力の邪魔になるので出さない。 */}
      {changing ? null : (
        <div className="sticky bottom-[calc(3.5rem+env(safe-area-inset-bottom))] z-20 space-y-1.5 pb-3 md:bottom-0 md:pb-[max(1rem,env(safe-area-inset-bottom))]">
          {own ? (
            <Link
              href={`/injury/${part}/new`}
              className="block rounded-full bg-white py-3 text-center text-sm font-semibold text-blue-700 shadow-lg shadow-slate-950/60 transition hover:bg-blue-50 active:scale-95"
            >
              {selectedLabel}に投稿
            </Link>
          ) : (
            <>
              <span className="block rounded-full border border-white/25 py-3 text-center text-sm font-semibold text-slate-300">
                この段階には投稿できません
              </span>
              <p className="text-center text-xs text-slate-300">投稿できるのは、いまの段階({injuryStages.find((i) => i.value === mine)?.label})だけです。</p>
            </>
          )}
        </div>
      )}

      <div className="space-y-2 border-t border-white/15 pt-4">
        {leaving === "idle" ? (
          <button
            type="button"
            onClick={() => setLeaving("confirm")}
            className="text-sm font-semibold text-slate-300 underline-offset-2 hover:underline"
          >
            このグループを抜ける
          </button>
        ) : (
          <div className="space-y-2 rounded-xl border border-white/20 glass bg-white/8 p-3 text-sm text-slate-200">
            <p>
              {injuryBodyPartLabel(group.body_part)}のグループを抜けますか?段階の記録は消えます。投稿は残ります。
            </p>
            <div className="flex gap-2">
              <Button type="button" variant="danger" disabled={leaving === "busy"} onClick={() => void leaveGroup()}>
                {leaving === "busy" ? "抜けています…" : "抜ける"}
              </Button>
              <Button type="button" variant="secondary" disabled={leaving === "busy"} onClick={() => setLeaving("idle")}>
                やめる
              </Button>
            </div>
            {leaveError ? <p className="text-red-400">{leaveError}</p> : null}
          </div>
        )}
      </div>
    </div>
  );
}
