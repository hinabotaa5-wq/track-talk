"use client";

import { useEffect, useRef, useState } from "react";
import { ChevronDown, Sparkles } from "lucide-react";
import { Button } from "./ui/button";
import { apiFetch, ApiRequestError } from "@/lib/api";
import { useActionLock } from "@/lib/use-action-lock";
import { senseDayLabel, senseSummaryMinNotes, senseSummaryRangeLabel, type SenseSummary, type SenseSummaryResponse, type SenseSummaryHistory } from "@/lib/sense-notes";

function SummaryList({ title, items }: { title: string; items: string[] }) {
  if (items.length === 0) return null;
  return (
    <div>
      <h3 className="text-xs font-semibold text-slate-300">{title}</h3>
      <ul className="mt-1 list-disc space-y-0.5 pl-5 text-sm text-slate-100">
        {items.map((item, i) => (
          <li key={i}>{item}</li>
        ))}
      </ul>
    </div>
  );
}

function SummaryEntry({ summary }: { summary: SenseSummary }) {
  return (
    <details className="group rounded-xl border border-white/15 bg-white/5">
      <summary className="flex cursor-pointer list-none items-start gap-3 p-4 [&::-webkit-details-marker]:hidden">
        <div className="min-w-0 flex-1 space-y-1">
          <span className="block text-xs text-slate-300">{senseSummaryRangeLabel(summary)} · {summary.note_count}個の感覚</span>
          <span className="line-clamp-2 text-sm leading-relaxed group-open:hidden">{summary.summary.overview}</span>
          <span className="block text-xs text-sky-200 group-open:hidden">タップしてふりかえる</span>
        </div>
        <ChevronDown className="mt-1 h-4 w-4 shrink-0 text-slate-300 transition-transform group-open:rotate-180" aria-hidden />
      </summary>
      <div className="space-y-4 px-4 pb-4">
        <p className="whitespace-pre-wrap text-sm leading-relaxed">{summary.summary.overview}</p>
        <SummaryList title="つかめてきた感覚" items={summary.summary.good} />
        <SummaryList title="気になる感覚" items={summary.summary.concerns} />
        <SummaryList title="次に意識すること" items={summary.summary.next} />
        <div className="space-y-3 border-t border-white/10 pt-4">
          <h3 className="text-sm font-semibold">もとになった感覚</h3>
          {summary.notes?.length ? (
            <ol className="space-y-3">
              {summary.notes.map((note) => (
                <li key={note.id} className="rounded-lg bg-black/15 p-3">
                  <p className="mb-1 text-xs text-slate-300">{senseDayLabel(note.day)}{note.event_label ? ` · ${note.event_label}` : ""}</p>
                  <p className="whitespace-pre-wrap break-words text-sm leading-relaxed">{note.body}</p>
                </li>
              ))}
            </ol>
          ) : <p className="text-xs text-slate-300">このまとめは、もとになった感覚を保存する前に作成されたものです。</p>}
        </div>
        <p className="text-xs text-slate-400">AI がまとめた目安です。</p>
      </div>
    </details>
  );
}

// 作成ボタンと過去のまとめ。元の感覚は作成時の内容で保存されている。
export function SenseSummaryCard({ token, total }: { token: string | null; total: number }) {
  const [summaries, setSummaries] = useState<SenseSummary[]>([]);
  const [cursor, setCursor] = useState<string | null>(null);
  const [pendingNotes, setPendingNotes] = useState(0);
  const [minNotes, setMinNotes] = useState(senseSummaryMinNotes);
  const [loaded, setLoaded] = useState(false);
  const [loadingMore, setLoadingMore] = useState(false);
  const [error, setError] = useState("");
  const [reload, setReload] = useState(0);
  const version = useRef(0);
  const { pending: busy, acquire, release } = useActionLock();

  useEffect(() => {
    if (!token) return;
    const controller = new AbortController();
    const requestedVersion = ++version.current;
    Promise.all([
      apiFetch<SenseSummaryResponse>("/v1/me/sense-notes/summary", { token, signal: controller.signal }),
      apiFetch<SenseSummaryHistory>("/v1/me/sense-notes/summaries", { token, signal: controller.signal }),
    ]).then(([progress, history]) => {
      if (controller.signal.aborted || requestedVersion !== version.current) return;
      setPendingNotes(progress.pending_notes ?? 0);
      setMinNotes(progress.min_notes ?? senseSummaryMinNotes);
      setSummaries(history.summaries);
      setCursor(history.next_cursor);
      setError("");
      setLoaded(true);
    }).catch(() => {
      if (!controller.signal.aborted && requestedVersion === version.current) setError("まとめを読み込めませんでした");
    });
    return () => controller.abort();
  }, [token, total, reload]);

  async function create() {
    if (!acquire()) return;
    setError("");
    try {
      const d = await apiFetch<SenseSummaryResponse>("/v1/me/sense-notes/summary", { method: "POST", token });
      // 作成前に始まった読み込みで、新しいまとめを上書きしない。
      version.current++;
      if (d.summary) {
        const created = d.summary;
        setSummaries((list) => [created, ...list.filter((item) => item.id !== created.id)]);
      }
      setPendingNotes(d.pending_notes ?? 0);
    } catch (e) {
      setError(e instanceof ApiRequestError ? e.message : "まとめを作れませんでした");
    } finally {
      release();
    }
  }

  async function loadMore() {
    if (!cursor || loadingMore) return;
    const requestedVersion = version.current;
    setLoadingMore(true);
    setError("");
    try {
      const d = await apiFetch<SenseSummaryHistory>(`/v1/me/sense-notes/summaries?cursor=${encodeURIComponent(cursor)}`, { token });
      if (requestedVersion !== version.current) return;
      setSummaries((list) => [...list, ...d.summaries.filter((item) => !list.some((existing) => existing.id === item.id))]);
      setCursor(d.next_cursor);
    } catch {
      if (requestedVersion === version.current) setError("過去のまとめを読み込めませんでした。もう一度お試しください");
    } finally {
      setLoadingMore(false);
    }
  }

  const shortBy = Math.max(0, minNotes - pendingNotes);
  return (
    <section className="space-y-3 rounded-2xl border border-white/15 bg-white/6 p-4" aria-label="まとめた感覚">
      <div className="flex flex-wrap items-center justify-between gap-3">
        <h2 className="flex items-center gap-1.5 text-sm font-semibold">
          <Sparkles className="h-4 w-4 text-amber-200" aria-hidden />
          まとめた感覚
        </h2>
        {loaded ? shortBy > 0 ? (
          <span className="text-xs text-slate-300">あと{shortBy}個でまとめられます</span>
        ) : (
          <Button variant="secondary" className="px-3 py-1 text-xs" onClick={create} disabled={busy}>
            {busy ? "まとめています..." : "6個の感覚をまとめる"}
          </Button>
        ) : null}
      </div>
      {loaded && summaries.length === 0 ? <p className="text-xs text-slate-300">感覚が{minNotes}個たまったら、ここにまとめを残せます。</p> : null}
      {!loaded && !error ? <p className="text-xs text-slate-300">読み込み中...</p> : null}
      <div className="space-y-2">
        {summaries.map((summary) => <SummaryEntry key={summary.id} summary={summary} />)}
      </div>
      {cursor ? <Button variant="secondary" onClick={loadMore} disabled={loadingMore}>{loadingMore ? "読み込み中..." : "過去のまとめをもっと見る"}</Button> : null}
      {error ? <p role="alert" className="text-sm text-red-300">{error}</p> : null}
      {!loaded && error ? <Button variant="secondary" onClick={() => setReload((value) => value + 1)}>もう一度読み込む</Button> : null}
    </section>
  );
}
