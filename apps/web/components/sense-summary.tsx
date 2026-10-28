"use client";

import { useCallback, useEffect, useRef, useState, type ReactNode } from "react";
import { ChevronDown, Sparkles, X } from "lucide-react";
import { Button } from "./ui/button";
import { apiFetch, ApiRequestError } from "@/lib/api";
import { useActionLock } from "@/lib/use-action-lock";
import { todayInTokyo } from "@/lib/format";
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

export function SenseSummaryEntry({ summary, open = false }: { summary: SenseSummary; open?: boolean }) {
  return (
    <details open={open || undefined} className="group rounded-xl border border-white/15 bg-white/5">
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

// 作成と履歴の状態を共有し、感覚・記録のどちらのカレンダーにも反映する。
export function useSenseSummaries(token: string | null, total: number, calendarMonth?: string) {
  const [createdResult, setCreatedResult] = useState<{ token: string; summary: SenseSummary } | null>(null);
  const [summaries, setSummaries] = useState<SenseSummary[]>([]);
  const [cursor, setCursor] = useState<string | null>(null);
  const [pendingNotes, setPendingNotes] = useState(0);
  const [minNotes, setMinNotes] = useState(senseSummaryMinNotes);
  const [loadedToken, setLoadedToken] = useState<string | null>(null);
  const loaded = token !== null && loadedToken === token;
  const [loadingMore, setLoadingMore] = useState(false);
  const [error, setError] = useState("");
  const [reload, setReload] = useState(0);
  const version = useRef(0);
  const moreRequest = useRef(false);
  const invalidate = useCallback(() => { version.current++; }, []);
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
      setLoadedToken(token);
    }).catch(() => {
      if (!controller.signal.aborted && requestedVersion === version.current) setError("まとめを読み込めませんでした");
    });
    return () => { controller.abort(); invalidate(); };
  }, [token, total, reload, invalidate]);

  async function create() {
    if (!token || !acquire()) return;
    const requestedVersion = version.current;
    setCreatedResult(null);
    setError("");
    try {
      const d = await apiFetch<SenseSummaryResponse>("/v1/me/sense-notes/summary", { method: "POST", token });
      if (requestedVersion !== version.current) return;
      // 作成前に始まった読み込みで、新しいまとめを上書きしない。
      version.current++;
      if (d.summary) {
        const created = d.summary;
        setCreatedResult({ token, summary: created });
        setSummaries((list) => [created, ...list.filter((item) => item.id !== created.id)]);
      }
      setPendingNotes(d.pending_notes ?? 0);
    } catch (e) {
      if (requestedVersion !== version.current) return;
      setError(e instanceof ApiRequestError ? e.message : "まとめを作れませんでした");
    } finally {
      release();
    }
  }

  const loadMore = useCallback(async () => {
    if (!token || !cursor || moreRequest.current) return;
    moreRequest.current = true;
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
      moreRequest.current = false;
      setLoadingMore(false);
    }
  }, [token, cursor]);

  // 履歴は作成順。対象月より前に作られたまとめまで読むと、過去月の点も欠けない。
  // 元のメモを過去の日付で保存できるため、to_day では読み込みの終点を判断しない。
  useEffect(() => {
    const oldest = summaries[summaries.length - 1];
    if (!loaded || !calendarMonth || !cursor || !oldest || error || loadingMore) return;
    if (todayInTokyo(new Date(oldest.created_at)) < `${calendarMonth}-01`) return;
    // eslint-disable-next-line react-hooks/set-state-in-effect
    void loadMore();
  }, [loaded, calendarMonth, cursor, summaries, error, loadingMore, loadMore]);

  return { createdSummary: createdResult?.token === token ? createdResult.summary : null, summaries: loaded ? summaries : [], cursor, pendingNotes, minNotes, loaded, loadingMore, error, busy, create, loadMore, retry: () => setReload((value) => value + 1) };
}

// 瓶の横では作成状況とボタンだけを表示し、まとめの本文はカレンダーで読む。
export function SenseSummaryCard({ state, compact = false }: { state: ReturnType<typeof useSenseSummaries>; compact?: boolean }) {
  const { summaries, cursor, pendingNotes, minNotes, loaded, loadingMore, error, busy, create, loadMore, retry } = state;
  const shortBy = Math.max(0, minNotes - pendingNotes);
  return (
    <section className={compact ? "min-w-0 space-y-2" : "space-y-3 rounded-2xl border border-white/15 bg-white/6 p-4"} aria-label="AI要約">
      <div className={compact ? "space-y-1.5" : "flex flex-wrap items-center justify-between gap-3"}>
        <h2 className="flex items-center gap-1.5 text-sm font-semibold">
          <Sparkles className="h-4 w-4 text-amber-200" aria-hidden />
          AI要約
        </h2>
        {loaded ? shortBy > 0 ? (
          <span className="block text-xs text-slate-300">あと{shortBy}個でまとめられます</span>
        ) : (
          <Button variant="secondary" className="border-amber-300/50 bg-amber-300/20 px-3 py-1 text-xs text-amber-100 hover:bg-amber-300/30 hover:text-amber-50 active:bg-amber-300/40" onClick={create} disabled={busy}>
            {busy ? "まとめています..." : `${minNotes}個の感覚をまとめる`}
          </Button>
        ) : null}
      </div>
      {!compact && loaded && summaries.length === 0 ? <p className="text-xs text-slate-300">感覚が{minNotes}個たまったらAIで要約できます</p> : null}
      {!loaded && !error ? <p className="text-xs text-slate-300">読み込み中...</p> : null}
      {!compact ? <div className="space-y-2">
        {summaries.map((summary) => <SenseSummaryEntry key={summary.id} summary={summary} />)}
      </div> : null}
      {!compact && cursor ? <Button variant="secondary" onClick={loadMore} disabled={loadingMore}>{loadingMore ? "読み込み中..." : "過去のまとめをもっと見る"}</Button> : null}
      {error ? <p role="alert" className="text-sm text-red-300">{error}</p> : null}
      {!loaded && error ? <Button variant="secondary" onClick={retry}>もう一度読み込む</Button> : null}
    </section>
  );
}

// 作成後の本文だけを順に表示する。履歴は全文のまま。
function AnimatedSummary({ summary }: { summary: SenseSummary }) {
  const sections: [string, string[]][] = [["つかめてきた感覚", summary.summary.good], ["気になる感覚", summary.summary.concerns], ["次に意識すること", summary.summary.next]];
  const text = [summary.summary.overview, ...sections.flatMap(([title, items]) => items.length ? [`${title}\n${items.map((item) => `・${item}`).join("\n")}`] : [])].join("\n\n");
  const characters = Array.from(text);
  const [visible, setVisible] = useState(0);
  useEffect(() => {
    const reduceMotion = window.matchMedia?.("(prefers-reduced-motion: reduce)").matches;
    const length = Array.from(text).length;
    let frame: number;
    let start: number | undefined;
    function tick(now: number) {
      start ??= now;
      const count = reduceMotion ? length : Math.floor((now - start) / 12) + 1;
      setVisible(count);
      if (count < length) frame = requestAnimationFrame(tick);
    }
    frame = requestAnimationFrame(tick);
    return () => cancelAnimationFrame(frame);
  }, [text]);
  const done = visible >= characters.length;
  return <>
    <p className="text-xs text-slate-300">{senseSummaryRangeLabel(summary)} · {summary.note_count}個の感覚</p>
    <p aria-hidden="true" className="whitespace-pre-wrap break-words text-sm leading-7">{characters.slice(0, visible).join("")}{!done ? <span className="ml-0.5 text-amber-300">▍</span> : null}</p>
    <p className="sr-only" role="status">{done ? text : "AI要約を表示しています"}</p>
  </>;
}

export function SenseSummaryResult({ state, children }: { state: ReturnType<typeof useSenseSummaries>; children?: ReactNode }) {
  const target = useRef<HTMLDivElement>(null);
  const closeButton = useRef<HTMLButtonElement>(null);
  const trigger = useRef<HTMLElement | null>(null);
  const [dismissed, setDismissed] = useState<string | null>(null);
  const { busy, createdSummary } = state;
  const visible = busy || Boolean(createdSummary && createdSummary.id !== dismissed);
  function close() {
    if (busy || !createdSummary) return;
    setDismissed(createdSummary.id);

  }
  useEffect(() => {
    if (!visible) {
      if (!trigger.current) return;
      const frame = requestAnimationFrame(() => {
        const destination = trigger.current?.isConnected ? trigger.current : target.current?.querySelector<HTMLElement>("button");
        destination?.focus({ preventScroll: true });
      });
      return () => cancelAnimationFrame(frame);
    }
    trigger.current = document.activeElement instanceof HTMLElement ? document.activeElement : null;
    const frame = requestAnimationFrame(() => {
      target.current?.scrollIntoView({ behavior: window.matchMedia?.("(prefers-reduced-motion: reduce)").matches ? "instant" : "smooth", block: "start" });
      target.current?.querySelector<HTMLElement>('[role="dialog"]')?.focus({ preventScroll: true });
    });
    return () => cancelAnimationFrame(frame);
  }, [visible]);
  return <div ref={target} className="relative isolate scroll-mt-24">
    <div inert={visible || undefined}>{children}</div>
    {visible ? <section role="dialog" tabIndex={-1} aria-label="新しいAI要約" onKeyDown={(event) => { if (event.key === "Escape") close(); }} className="absolute inset-0 z-20 flex min-h-0 flex-col rounded-2xl border border-amber-200/30 bg-slate-950/65 p-4 shadow-xl backdrop-blur-[1px]">
      <div className="mb-3 flex shrink-0 items-center justify-between gap-3">
        <h2 className="flex items-center gap-2 text-sm font-semibold"><Sparkles className="size-4 text-amber-200" aria-hidden />AI要約</h2>
        <button ref={closeButton} type="button" disabled={busy} onClick={close} aria-label="AI要約を閉じる" className="flex size-9 items-center justify-center rounded-full text-slate-300 hover:bg-white/10 focus-visible:outline-2 focus-visible:outline-amber-300 disabled:opacity-40"><X className="size-4" aria-hidden /></button>
      </div>
      <div className="min-h-0 flex-1 space-y-3 overflow-y-auto overscroll-contain pr-1">
        {busy ? <p role="status" className="text-sm text-slate-300">まとめています…</p> : createdSummary ? <AnimatedSummary key={createdSummary.id} summary={createdSummary} /> : null}
      </div>
    </section> : null}
  </div>;
}
