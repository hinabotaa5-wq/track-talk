"use client";

import { useEffect, useLayoutEffect, useRef, useState } from "react";
import { ChevronLeft, ChevronRight, X } from "lucide-react";
import { SenseNoteItem } from "./sense-notes";
import { SenseSummaryEntry } from "./sense-summary";
import { Button } from "./ui/button";
import { RaceRecordRow } from "./race-records";
import { raceMarkKind, sortRaceRecords, type RaceRecord, type RaceRecords } from "@/lib/race-records";
import { todayInTokyo } from "@/lib/format";
import { senseDayLabel, type SenseNote, type SenseNoteStats, type SenseSummary } from "@/lib/sense-notes";

export function SenseNoteCalendar({ notes, token, month, onMonthChange, loading, failed, onRetry, onDeleted, records, selectedDay: controlledDay, onDayChange, onRecordsChanged, recordsLoading = false, recordsFailed = false, onRecordsRetry, summaries = [], summariesLoading = false, summariesFailed = false, onSummariesRetry }: {
  summaries?: SenseSummary[];
  summariesLoading?: boolean;
  summariesFailed?: boolean;
  onSummariesRetry?: () => void;
  notes: SenseNote[];
  records?: RaceRecord[];
  selectedDay?: string | null;
  onDayChange?: (day: string | null) => void;
  onRecordsChanged?: (records: RaceRecords) => void;
  recordsLoading?: boolean;
  recordsFailed?: boolean;
  onRecordsRetry?: () => void;
  token: string | null;
  month: string;
  onMonthChange: (month: string) => void;
  loading: boolean;
  failed: boolean;
  onRetry: () => void;
  onDeleted: (id: string, stats: SenseNoteStats) => void;
}) {
  const [localDay, setLocalDay] = useState<string | null>(null);
  const selectedDay = controlledDay === undefined ? localDay : controlledDay;
  const setSelectedDay = (day: string | null) => { setLocalDay(day); onDayChange?.(day); };
  const calendar = useRef<HTMLElement>(null);
  const [overlayPadding, setOverlayPadding] = useState({ top: 8, bottom: 8 });
  const closeButton = useRef<HTMLButtonElement>(null);
  const trigger = useRef<HTMLButtonElement | null>(null);
  const shared = records !== undefined;
  const title = shared ? "感覚と記録のカレンダー" : "感覚メモの履歴";
  const today = todayInTokyo();
  const [year, monthNumber] = month.split("-").map(Number);
  const offset = new Date(Date.UTC(year, monthNumber - 1, 1)).getUTCDay();
  const dayCount = new Date(Date.UTC(year, monthNumber, 0)).getUTCDate();
  const marked = new Set(notes.map((note) => note.day));
  const recorded = new Set(records?.map((record) => record.raced_on));
  const summarized = new Set(summaries.map((summary) => summary.to_day));
  const selectedRecords = sortRaceRecords(records?.filter((record) => record.raced_on === selectedDay) ?? []);
  const selectedNotes = notes.filter((note) => note.day === selectedDay);
  const selectedSummaries = summaries.filter((summary) => summary.to_day === selectedDay);
  const detailsOpen = Boolean(selectedDay && (selectedNotes.length || selectedRecords.length || selectedSummaries.length));

  // カレンダーが画面の途中にある場合も、詳細はヘッダー・下部バーに隠れない範囲に収める。
  useLayoutEffect(() => {
    if (!detailsOpen) return;
    function updateBounds() {
      const bounds = calendar.current?.getBoundingClientRect();
      if (!bounds) return;
      const headerBottom = Math.max(0, document.querySelector("header")?.getBoundingClientRect().bottom ?? 0);
      const nav = [...document.querySelectorAll<HTMLElement>('nav[aria-label="メイン"]')]
        .find((element) => getComputedStyle(element).position === "fixed" && element.getBoundingClientRect().height > 0);
      const bottom = nav?.getBoundingClientRect().top ?? window.innerHeight;
      const next = { top: Math.max(8, Math.ceil(headerBottom + 8 - bounds.top)), bottom: Math.max(8, Math.ceil(bounds.bottom - bottom + 8)) };
      setOverlayPadding((previous) => previous.top === next.top && previous.bottom === next.bottom ? previous : next);
    }
    updateBounds();
    window.addEventListener("resize", updateBounds);
    window.addEventListener("scroll", updateBounds, { passive: true });
    return () => { window.removeEventListener("resize", updateBounds); window.removeEventListener("scroll", updateBounds); };
  }, [detailsOpen]);

  useEffect(() => {
    if (!detailsOpen) return;
    closeButton.current?.focus({ preventScroll: true });
    return () => trigger.current?.focus({ preventScroll: true });
  }, [detailsOpen]);

  function closeDetails() {
    setSelectedDay(null);
  }

  function moveMonth(delta: number) {
    const date = new Date(Date.UTC(year, monthNumber - 1 + delta, 1));
    setSelectedDay(null);
    onMonthChange(date.toISOString().slice(0, 7));
  }

  return (
    <section ref={calendar} aria-label={title} className="relative isolate rounded-2xl border border-white/20 glass bg-white/10 p-4">
      <div className="space-y-4">
        {shared ? null : <h2 className="text-base font-bold">{title}</h2>}
        <div className="flex items-center justify-between">
          <Button variant="secondary" aria-label="前の月" onClick={() => moveMonth(-1)}><ChevronLeft className="size-5" aria-hidden /></Button>
          <h3 aria-live="polite" className="font-semibold tabular-nums">{year}年{monthNumber}月</h3>
          <Button variant="secondary" aria-label="次の月" disabled={month >= today.slice(0, 7)} onClick={() => moveMonth(1)}><ChevronRight className="size-5" aria-hidden /></Button>
        </div>
        <div className="grid grid-cols-7 gap-1 text-center" aria-busy={loading || recordsLoading}>
          {[..."日月火水木金土"].map((day, index) => <span key={day} className={`py-2 text-xs ${index === 0 ? "text-rose-300" : index === 6 ? "text-sky-300" : "text-slate-300"}`}>{day}</span>)}
          {Array.from({ length: offset }, (_, index) => <span key={`blank-${index}`} aria-hidden />)}
          {Array.from({ length: dayCount }, (_, index) => {
            const number = index + 1;
            const day = `${month}-${String(number).padStart(2, "0")}`;
            const written = marked.has(day);
            const hasRecord = recorded.has(day);
            const hasSummary = summarized.has(day);
            const hasContent = written || hasRecord || hasSummary;
            return (
              <button
                key={day}
                type="button"
                disabled={!hasContent}
                aria-label={`${year}年${monthNumber}月${number}日${written ? "、感覚メモあり" : "、感覚メモなし"}${shared ? hasRecord ? "、記録あり" : "、記録なし" : ""}${hasSummary ? "、まとめた感覚あり" : ""}`}
                aria-pressed={selectedDay === day}
                aria-current={day === today ? "date" : undefined}
                onClick={(event) => { trigger.current = event.currentTarget; setSelectedDay(day); }}
                className={`flex min-h-12 min-w-0 flex-col items-center justify-center gap-1 rounded-xl border text-sm tabular-nums transition focus-visible:outline-2 focus-visible:outline-sky-300 ${selectedDay === day ? "border-sky-300 bg-sky-400/25 text-white" : hasContent ? "border-sky-300/30 bg-sky-400/10 text-sky-100 hover:bg-sky-400/20" : "border-transparent text-slate-400"} ${day === today ? "ring-1 ring-inset ring-white/40" : ""}`}
              >
                <span>{number}</span>
                <span className="flex h-1.5 items-center justify-center gap-1" aria-hidden>
                  {written ? <span className="size-1.5 rounded-full bg-sky-300" /> : null}
                  {shared && hasRecord ? <span className="size-1.5 rounded-full bg-amber-300" /> : null}
                  {hasSummary ? <span className="size-1.5 rounded-full bg-violet-300" /> : null}
                </span>
              </button>
            );
          })}
        </div>
        {shared ? <p className="flex flex-wrap gap-x-4 gap-y-1 text-xs text-slate-300"><span className="flex items-center gap-1"><span aria-hidden className="size-1.5 rounded-full bg-sky-300" />感覚メモ</span><span className="flex items-center gap-1"><span aria-hidden className="size-1.5 rounded-full bg-amber-300" />記録</span></p> : null}
        {summaries.length ? <p className="flex items-center gap-1 text-xs text-slate-300"><span aria-hidden className="size-1.5 rounded-full bg-violet-300" />まとめた感覚（対象期間の最終日）</p> : null}
        {summariesLoading ? <p role="status" className="text-sm text-slate-300">まとめた感覚を読み込み中...</p> : summariesFailed ? <div role="alert"><p className="text-sm text-red-300">まとめた感覚を読み込めませんでした。</p><Button variant="secondary" onClick={onSummariesRetry}>まとめを再読み込み</Button></div> : null}
        {recordsLoading ? <p role="status" className="text-sm text-slate-300">記録を読み込み中...</p> : recordsFailed ? <div role="alert"><p className="text-sm text-red-300">記録を読み込めませんでした。</p><Button variant="secondary" onClick={onRecordsRetry}>記録を再読み込み</Button></div> : null}
        {loading ? <p role="status" className="text-sm text-slate-300">この月のメモを読み込み中...</p> : failed ? <div role="alert" className="space-y-2"><p className="text-sm text-red-300">過去のメモを読み込めませんでした。</p><Button variant="secondary" onClick={onRetry}>再読み込み</Button></div> : !notes.some((note) => note.day.startsWith(month)) && !records?.some((record) => record.raced_on.startsWith(month)) && !summaries.some((summary) => summary.to_day.startsWith(month)) && !recordsLoading && !recordsFailed && !summariesLoading && !summariesFailed ? <p className="text-sm text-slate-300">{shared ? "この月の感覚メモ・記録はありません。" : "この月の感覚メモはありません。"}</p> : null}
      </div>
      {detailsOpen && selectedDay ? (
        <div className="absolute inset-0 z-20 flex items-center justify-center rounded-2xl bg-slate-950/60 px-2 backdrop-blur-sm" style={{ paddingTop: overlayPadding.top, paddingBottom: overlayPadding.bottom }} onClick={(event) => { if (event.target === event.currentTarget) closeDetails(); }}>
          <section role="dialog" aria-label={`${senseDayLabel(selectedDay)}の詳細`} onKeyDown={(event) => { if (event.key === "Escape") { event.stopPropagation(); closeDetails(); } }} className="flex max-h-full w-full min-w-0 flex-col overflow-hidden rounded-xl border border-white/25 bg-slate-900 shadow-xl">
            <div className="flex shrink-0 items-center justify-between gap-2 border-b border-white/15 px-3 py-1">
              <h3 className="text-sm font-semibold">{senseDayLabel(selectedDay)}</h3>
              <button type="button" ref={closeButton} aria-label="詳細を閉じる" onClick={closeDetails} className="flex size-10 items-center justify-center rounded-full hover:bg-white/10"><X className="size-4" aria-hidden /></button>
            </div>
            <div className="min-h-0 space-y-2 overflow-y-auto overscroll-contain p-3">
              {selectedSummaries.length ? <h4 className="text-sm font-semibold text-violet-200">まとめた感覚</h4> : null}
              {selectedSummaries.map((summary) => <SenseSummaryEntry key={summary.id} summary={summary} open />)}
              {selectedRecords.length ? <h4 className="text-sm font-semibold text-amber-200">記録</h4> : null}
              {selectedRecords.map((record) => <RaceRecordRow key={record.id} record={record} kind={raceMarkKind(record.event)} token={token} onChanged={(next) => onRecordsChanged?.(next)} />)}
              {shared && selectedNotes.length ? <h4 className="text-sm font-semibold text-sky-200">感覚メモ</h4> : null}
              {selectedNotes.map((note) => <SenseNoteItem key={note.id} note={note} token={token} onDeleted={onDeleted} />)}
            </div>
          </section>
        </div>
      ) : null}
    </section>
  );
}
