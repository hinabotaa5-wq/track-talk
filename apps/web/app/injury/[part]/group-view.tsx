"use client";

import Link from "next/link";
import { useRouter } from "next/navigation";
import { useCallback, useEffect, useState } from "react";
import { FeedCard, feedGridClass } from "@/components/feed-card";
import { FeedLoadingFrame } from "@/components/loading-frame";
import { EmptyState, ErrorState } from "@/components/empty-state";
import { useGuardedLinkClick } from "@/components/leave-guard";
import { InjuryGoalCelebration } from "@/components/injury-goal-celebration";
import { InjuryJoinWizard } from "@/components/injury-join-wizard";
import { InjuryRoadmap } from "@/components/injury-roadmap";
import { Button } from "@/components/ui/button";
import { useAuth } from "@/components/auth-provider";
import { apiFetch, ApiRequestError } from "@/lib/api";
import type { PostsPage } from "@/lib/fetch-posts";
import {
  canPostInStage,
  injuryBodyPartLabel,
  injuryCelebrationKey,
  injuryStages,
  shouldCelebrate,
  type InjuryGroup,
  type InjuryStage,
} from "@/lib/injury";

type Load = { kind: "loading" } | { kind: "joined"; group: InjuryGroup } | { kind: "not-joined" } | { kind: "failed" };

function celebrated(part: string) {
  try {
    return localStorage.getItem(injuryCelebrationKey(part)) === "1";
  } catch {
    return false;
  }
}

function markCelebrated(part: string) {
  try {
    localStorage.setItem(injuryCelebrationKey(part), "1");
  } catch {
    // 保存できなくても、祝うのが1回余計に出るだけ。
  }
}

export function GroupView({ part }: { part: string }) {
  const { session, token, loading } = useAuth();
  const [load, setLoad] = useState<Load>({ kind: "loading" });
  const [selected, setSelected] = useState<InjuryStage | null>(null);
  const [posts, setPosts] = useState<{ stage: string; page: PostsPage } | null>(null);
  const router = useRouter();
  const guardedClick = useGuardedLinkClick();
  const [changing, setChanging] = useState(false);
  const [applying, setApplying] = useState<InjuryStage | null>(null);
  const [leaving, setLeaving] = useState<"idle" | "confirm" | "busy">("idle");
  const [leaveError, setLeaveError] = useState("");
  const [changeError, setChangeError] = useState("");
  const [celebrate, setCelebrate] = useState(false);

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
    apiFetch<PostsPage>(`/v1/injury/groups/${part}/posts?per_page=20&stage=${selected}`, { token, signal: controller.signal })
      .then((page) => setPosts({ stage: selected, page: { posts: page.posts ?? [], meta: page.meta } }))
      .catch(() => {
        if (!controller.signal.aborted) setPosts({ stage: selected, page: { posts: [], meta: { page: 1, per_page: 20, total: 0 } } });
      });
    return () => controller.abort();
  }, [joined, part, token, selected, myStage]);

  async function changeStage(stage: InjuryStage) {
    if (load.kind !== "joined" || stage === load.group.me.stage) {
      setChanging(false);
      return;
    }
    if (applying) return;
    setChangeError("");
    setApplying(stage);
    try {
      await apiFetch(`/v1/injury/groups/${part}/me`, { method: "PATCH", token, body: JSON.stringify({ stage }) });
      setChanging(false);
      setSelected(stage);
      setPosts(null);
      await fetchGroup();
      if (shouldCelebrate(stage, celebrated(part))) {
        markCelebrated(part);
        setCelebrate(true);
      }
    } catch (e) {
      setChangeError(e instanceof ApiRequestError ? e.message : "段階を変えられませんでした");
    } finally {
      setApplying(null);
    }
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

  const back = (
    <Link
      href="/injury"
      onClick={(event) => guardedClick(event, "/injury")}
      className="inline-block text-sm font-semibold text-blue-300 underline-offset-2 hover:underline"
    >
      ← ケガ
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
            setSelected(null);
            setPosts(null);
            void fetchGroup();
          }}
        />
      </div>
    );
  }

  if (load.kind === "loading" || !selected) {
    return (
      <div className="space-y-4">
        {back}
        <FeedLoadingFrame />
      </div>
    );
  }

  const { group } = load;
  const mine = group.me.stage;
  const own = canPostInStage(selected, mine);
  const selectedLabel = injuryStages.find((item) => item.value === selected)?.label ?? "";
  const current = posts?.stage === selected ? posts.page : null;

  return (
    <div className="space-y-4">
      {back}
      <div className="flex items-start justify-between gap-3">
        <div>
          <h1 className="text-2xl font-bold">
            {injuryBodyPartLabel(group.body_part)}の仲間 · {group.members}人
          </h1>
          <p className="mt-1 text-sm text-slate-200">一緒に乗り越えよう。どの段階もタップして読めます。</p>
        </div>
        <Button variant="secondary" className="shrink-0" onClick={() => setChanging((v) => !v)} aria-expanded={changing}>
          段階を変える
        </Button>
      </div>

      {celebrate ? <InjuryGoalCelebration onClose={() => setCelebrate(false)} /> : null}

      {changing ? (
        <div className="space-y-2 rounded-2xl border border-white/20 glass bg-white/10 p-3" role="radiogroup" aria-label="いまの段階">
          {injuryStages.map((item) => (
            <button
              key={item.value}
              type="button"
              role="radio"
              aria-checked={item.value === mine}
              onClick={() => void changeStage(item.value)}
              disabled={applying !== null}
              className={`w-full rounded-xl border px-4 py-2.5 text-left text-sm font-semibold transition active:scale-[0.98] disabled:cursor-wait ${
                item.value === applying
                  ? "border-blue-400 bg-blue-950/60 text-white"
                  : item.value === mine
                    ? "border-blue-400 bg-blue-950/60 text-white"
                    : "border-white/20 bg-white/8 text-slate-100 hover:bg-white/14"
              } ${applying !== null && item.value !== applying ? "opacity-50" : ""}`}
            >
              {item.label}
              {item.value === applying ? "に変更しています…" : item.value === mine ? "(いま)" : ""}
            </button>
          ))}
          {applying ? (
            <p role="status" className="text-sm text-slate-300">
              段階を変えています。少し待ってください。
            </p>
          ) : null}
          {changeError ? <p className="text-sm text-red-400">{changeError}</p> : null}
        </div>
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

      <div className="sticky bottom-20 space-y-1.5">
        {own ? (
          <Link
            href={`/injury/${part}/new`}
            className="block rounded-full bg-blue-600 py-3 text-center text-sm font-semibold text-white shadow-xs transition hover:bg-blue-500 active:scale-95"
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
              {injuryBodyPartLabel(group.body_part)}のグループを抜けます。段階の記録は消えて、グループの中は読めなくなります。書いた投稿は残ります(投稿ごとに削除できます)。あとで、もう一度入れます。
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
