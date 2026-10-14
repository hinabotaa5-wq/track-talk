"use client";

import Link from "next/link";
import { useRouter } from "next/navigation";
import { useCallback, useEffect, useState } from "react";
import { useAuth } from "@/components/auth-provider";
import { Button } from "@/components/ui/button";
import { Textarea } from "@/components/ui/textarea";
import { apiFetch, ApiRequestError } from "@/lib/api";
import {
  archiveTimelineText,
  injuryArchiveStatuses,
  injuryStageLabelOf,
  maskWordsError,
  parseMaskWords,
  type InjuryArchive,
  type InjuryArchiveStatus,
} from "@/lib/injury";
import type { Meta } from "@/lib/types";

const perPage = 20;

function ArchiveCard({
  archive,
  token,
  onChanged,
}: {
  archive: InjuryArchive;
  token: string | null;
  onChanged: () => void;
}) {
  const [words, setWords] = useState("");
  const [error, setError] = useState("");
  const [busy, setBusy] = useState(false);

  async function run(action: () => Promise<unknown>) {
    setBusy(true);
    setError("");
    try {
      await action();
      onChanged();
    } catch (e) {
      setError(e instanceof ApiRequestError ? (e.fields ? Object.values(e.fields)[0] : e.message) : "失敗しました");
    } finally {
      setBusy(false);
    }
  }

  const review = (status: InjuryArchiveStatus) =>
    run(() => apiFetch(`/v1/admin/injury-archives/${archive.id}`, { method: "PATCH", token, body: JSON.stringify({ review_status: status }) }));

  async function mask() {
    const list = parseMaskWords(words);
    const problem = maskWordsError(list);
    if (problem) {
      setError(problem);
      return;
    }
    await run(async () => {
      await apiFetch(`/v1/admin/injury-archives/${archive.id}/mask`, { method: "POST", token, body: JSON.stringify({ words: list }) });
      setWords("");
    });
  }

  async function remove() {
    if (!window.confirm("この記録を削除します。元に戻せません。")) return;
    await run(() => apiFetch(`/v1/admin/injury-archives/${archive.id}`, { method: "DELETE", token }));
  }

  return (
    <li className="space-y-3 rounded-xl border border-white/20 glass bg-white/10 p-3">
      <p className="text-xs text-slate-200">
        {archive.body_part_label} · 痛めた月 {archive.injured_month} · 残した月 {archive.archived_month}
      </p>
      {archive.stage_timeline.length > 0 ? (
        <p className="text-sm text-slate-100">{archiveTimelineText(archive.stage_timeline)}</p>
      ) : null}
      {archive.place ? <p className="whitespace-pre-wrap text-sm">場所: {archive.place}</p> : null}
      {archive.situation ? <p className="whitespace-pre-wrap text-sm">状況: {archive.situation}</p> : null}
      {archive.posts.map((post, index) => (
        <div key={index} className="space-y-1 rounded-lg border border-white/15 bg-white/6 p-2">
          <p className="text-xs text-slate-300">投稿{post.stage ? ` · ${injuryStageLabelOf(post.stage)}` : ""}</p>
          {post.title ? <p className="text-sm font-medium">{post.title}</p> : null}
          <p className="whitespace-pre-wrap text-sm text-slate-100">{post.body}</p>
          {post.updates.map((update, i) => (
            <p key={i} className="whitespace-pre-wrap border-l border-white/25 pl-2 text-sm text-slate-200">
              経過 · {injuryStageLabelOf(update.stage)}: {update.body}
            </p>
          ))}
        </div>
      ))}

      <div className="space-y-2">
        <Textarea
          rows={2}
          aria-label="伏せる言葉"
          placeholder="名前や地名など、伏せる言葉を改行かカンマで区切って入力"
          value={words}
          onChange={(event) => {
            setWords(event.target.value);
            setError("");
          }}
        />
        <div className="flex flex-wrap gap-2">
          <Button type="button" variant="secondary" disabled={busy} onClick={() => void mask()}>
            伏せる(確認待ちに戻る)
          </Button>
          {archive.review_status !== "approved" ? (
            <Button type="button" disabled={busy} onClick={() => void review("approved")}>
              承認
            </Button>
          ) : null}
          {archive.review_status !== "rejected" ? (
            <Button type="button" variant="secondary" disabled={busy} onClick={() => void review("rejected")}>
              却下
            </Button>
          ) : (
            <>
              <Button type="button" variant="secondary" disabled={busy} onClick={() => void review("pending")}>
                確認待ちに戻す
              </Button>
              <Button type="button" variant="danger" disabled={busy} onClick={() => void remove()}>
                削除
              </Button>
            </>
          )}
        </div>
        {error ? <p className="text-sm text-red-400">{error}</p> : null}
      </div>
    </li>
  );
}

export default function InjuryArchivesPage() {
  const { session, profile, token, loading } = useAuth();
  const router = useRouter();
  const isAdmin = Boolean(profile?.is_admin);
  const [status, setStatus] = useState<InjuryArchiveStatus>("pending");
  const [items, setItems] = useState<InjuryArchive[]>([]);
  const [total, setTotal] = useState(0);
  const [page, setPage] = useState(1);
  const [reload, setReload] = useState(0);
  const [error, setError] = useState("");
  const [loadedKey, setLoadedKey] = useState("");

  const key = `${status}:${page}:${reload}`;

  useEffect(() => {
    if (loading) return;
    if (!session) router.replace("/login");
    else if (!profile?.is_admin) router.replace("/");
  }, [loading, session, profile, router]);

  useEffect(() => {
    if (!isAdmin || !token) return;
    let cancelled = false;
    apiFetch<{ archives: InjuryArchive[]; meta: Meta }>(
      `/v1/admin/injury-archives?status=${status}&page=${page}&per_page=${perPage}`,
      { token },
    )
      .then((data) => {
        if (cancelled) return;
        const rows = data.archives ?? [];
        setItems((current) => (page === 1 ? rows : [...current, ...rows]));
        setTotal(data.meta.total);
        setError("");
        setLoadedKey(key);
      })
      .catch(() => {
        if (cancelled) return;
        setError("取得に失敗しました");
        setLoadedKey(key);
      });
    return () => {
      cancelled = true;
    };
  }, [isAdmin, token, status, page, reload, key]);

  const changed = useCallback(() => {
    setPage(1);
    setReload((n) => n + 1);
  }, []);

  if (loading || !isAdmin) {
    return <p className="text-sm text-slate-200">読み込み中...</p>;
  }

  const busy = loadedKey !== key;

  return (
    <div className="mx-auto max-w-2xl space-y-4">
      <Link href="/admin" className="inline-block text-sm font-semibold text-blue-300 underline-offset-2 hover:underline">
        ← 管理者画面
      </Link>
      <div>
        <h1 className="text-2xl font-bold">ケガの匿名記録の確認</h1>
        <p className="mt-1 text-sm text-slate-300">
          退会に同意した人の記録です。名前や地名など、個人が特定できそうな言葉が残っていないかを読んで確かめ、あれば伏せてから承認してください。承認した記録だけを、分析に使います。
        </p>
      </div>
      <div role="tablist" className="flex flex-wrap gap-2">
        {injuryArchiveStatuses.map((item) => (
          <button
            key={item.value}
            type="button"
            role="tab"
            aria-selected={status === item.value}
            onClick={() => {
              setStatus(item.value);
              setPage(1);
              setItems([]);
            }}
            className={`rounded-full border px-4 py-2 text-sm font-medium transition active:scale-95 ${
              status === item.value ? "border-blue-400 bg-blue-500/30 text-white" : "border-white/25 bg-white/10 text-slate-200"
            }`}
          >
            {item.label}
          </button>
        ))}
      </div>
      {error ? <p className="text-sm text-red-400">{error}</p> : null}
      <p className="text-sm text-slate-300">{total}件</p>
      <ul className="space-y-3">
        {items.map((archive) => (
          <ArchiveCard key={archive.id} archive={archive} token={token} onChanged={changed} />
        ))}
      </ul>
      {!busy && items.length === 0 && !error ? <p className="text-sm text-slate-200">まだありません</p> : null}
      {items.length < total ? (
        <button
          type="button"
          onClick={() => setPage((p) => p + 1)}
          disabled={busy}
          className="w-full rounded-xl border border-white/25 bg-white/10 py-2 text-sm text-slate-100 disabled:opacity-50"
        >
          {busy ? "読み込み中..." : "もっと見る"}
        </button>
      ) : null}
    </div>
  );
}
