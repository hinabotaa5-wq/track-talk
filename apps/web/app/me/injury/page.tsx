"use client";

import { useRouter } from "next/navigation";
import { useEffect, useRef, useState } from "react";
import { useAuth } from "@/components/auth-provider";
import { EmptyState, ErrorState } from "@/components/empty-state";
import { LoadMore } from "@/components/load-more";
import { InjuryProgressCompare } from "@/components/injury-progress-compare";
import type { Meta } from "@/lib/types";
import { apiFetch, ApiRequestError } from "@/lib/api";
import { formatDateTime } from "@/lib/format";
import { clearHomeFeeds, clearSavedFeeds } from "@/lib/home-feed-cache";
import { injuryHistoryStageText, type InjuryHistoryItem } from "@/lib/injury";
import { deleteConfirmOptions, useConfirm } from "@/components/confirm-dialog";

export default function MyInjuryHistoryPage() {
  const { session, token, sessionLoading: loading } = useAuth();
  const router = useRouter();
  const [rawItems, setItems] = useState<InjuryHistoryItem[] | null>(null);
  const [owner, setOwner] = useState<string | null>(null);
  const items = owner === token ? rawItems : null;
  const generation = useRef(0);
  const [repeated, setRepeated] = useState<{ label: string; count: number }[]>([]);
  const [next, setNext] = useState("");
  const [moreError, setMoreError] = useState("");
  const [loadingMore, setLoadingMore] = useState(false);
  const [expired, setExpired] = useState(false);
  const requestPending = useRef(false);
  const [reload, setReload] = useState(0);
  const [failed, setFailed] = useState(false);
  const [deletingId, setDeletingId] = useState<string | null>(null);
  const confirmDialog = useConfirm();

  useEffect(() => {
    if (!loading && !session) router.replace("/login?next=/me/injury");
  }, [loading, session, router]);

  useEffect(() => {
    if (!token) return;
    let active = true;
    const effectGeneration = ++generation.current;
    apiFetch<{ items: InjuryHistoryItem[]; repeated: { label: string; count: number }[]; meta: Meta }>("/v1/me/injury-history?pagination=cursor&per_page=24", { token })
      .then((d) => {
        if (!active) return;
        setFailed(false);
        setOwner(token);
        setItems(d.items ?? []);
        setRepeated(d.repeated ?? []);
        setNext(d.meta?.has_more ? d.meta.next_cursor ?? "" : "");
        setExpired(false);
        setMoreError("");
      })
      .catch(() => {
        if (active) setFailed(true);
      });
    return () => {
      active = false;
      generation.current = effectGeneration + 1;
    };
  }, [token, reload]);

  async function loadMore() {
    if (!next || requestPending.current) return;
    requestPending.current = true;
    setLoadingMore(true);
    const cursor = next;
    const requestGeneration = generation.current;
    try {
      const d = await apiFetch<{ items: InjuryHistoryItem[]; meta: Meta }>(`/v1/me/injury-history?pagination=cursor&per_page=24&cursor=${encodeURIComponent(cursor)}`, { token });
      if (requestGeneration !== generation.current) return;
      setItems((current) => {
        const seen = new Set(current?.map((item) => item.id));
        return [...(current ?? []), ...d.items.filter((item) => !seen.has(item.id))];
      });
      setNext(d.meta.has_more ? d.meta.next_cursor ?? "" : "");
      setMoreError("");
    } catch (error) {
      if (requestGeneration !== generation.current) return;
      setMoreError("続きを読み込めませんでした");
      setExpired(error instanceof ApiRequestError && error.code === "invalid_cursor");
    } finally {
      requestPending.current = false;
      setLoadingMore(false);
    }
  }

  async function remove(id: string) {
    if (!(await confirmDialog(deleteConfirmOptions("この投稿")))) return;
    setDeletingId(id);
    try {
      await apiFetch(`/v1/posts/${id}`, { method: "DELETE", token });
    } catch (e) {
      setDeletingId(null);
      alert(e instanceof ApiRequestError ? e.message : "削除できませんでした");
      return;
    }
    clearHomeFeeds();
    clearSavedFeeds();
    setItems((current) => current?.filter((item) => item.id !== id) ?? current);
    setDeletingId(null);
    setReload((value) => value + 1);
  }

  if (loading || !session) return <p className="text-sm text-slate-200">読み込み中...</p>;

  return (
    <div className="space-y-4">
      <div>
        <h1 className="text-2xl font-bold">自分のケガ履歴</h1>
        <p className="mt-1 text-sm text-slate-200">
          これまでのケガの投稿と、経過を報告した段階ごとの日数を振り返れます。あなただけに表示されます。
        </p>
      </div>

      {failed ? (
        <ErrorState message="履歴の取得に失敗しました。APIの起動を確認してください。" />
      ) : items === null ? (
        <p className="text-sm text-slate-200">読み込み中...</p>
      ) : items.length === 0 ? (
        <EmptyState title="まだケガの投稿がありません" body="ケガのグループに投稿すると、ここに履歴がたまります。" />
      ) : (
        <>
          {repeated.length > 0 ? (
            <p className="rounded-xl border border-amber-700/60 bg-amber-950/40 p-3 text-sm text-amber-200">
              {repeated.map((item) => `${item.label}（${item.count}回）`).join("、")}は、これまでに複数回ケガの投稿があります。
            </p>
          ) : null}
          <ul className="space-y-3">
            {items.map((item) => (
              <li key={item.id} className="rounded-xl border border-white/20 glass bg-white/10 p-4">
                <div className="flex items-start gap-3">
                  <div className="min-w-0 flex-1 space-y-2">
                    <p className="flex flex-wrap items-center gap-2 text-xs">
                      <span className="rounded-full border border-white/20 bg-white/12 px-2.5 py-0.5 font-semibold text-slate-200">
                        {item.body_part_label}
                      </span>
                      <span className="text-slate-300">{formatDateTime(item.created_at)}</span>
                    </p>
                    <h2 className="font-semibold text-slate-100">{item.title}</h2>
                    {item.stages.length > 0 ? (
                      <ol className="flex flex-wrap gap-x-3 gap-y-1 text-sm text-emerald-300">
                        {item.stages.map((stage) => (
                          <li key={stage.stage}>{injuryHistoryStageText(stage)}</li>
                        ))}
                      </ol>
                    ) : (
                      <p className="text-sm text-slate-300">経過の報告はまだありません。</p>
                    )}
                  </div>
                  <button
                    type="button"
                    onClick={() => remove(item.id)}
                    disabled={deletingId !== null}
                    className="shrink-0 rounded-lg border border-red-400/40 px-3 py-1.5 text-sm font-semibold text-red-300 hover:bg-red-950/40 disabled:opacity-50"
                  >
                    削除
                  </button>
                </div>
              </li>
            ))}
          </ul>
          {next ? <LoadMore onLoad={() => void loadMore()} loading={loadingMore} error={moreError} expired={expired} /> : null}
          {token ? <InjuryProgressCompare token={token} /> : null}
        </>
      )}
    </div>
  );
}
