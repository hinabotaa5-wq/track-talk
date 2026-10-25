"use client";

import { useState } from "react";
import { ChevronLeft, ChevronRight, X } from "lucide-react";
import { SenseNoteItem } from "./sense-notes";
import { Button } from "./ui/button";
import { RaceRecordRow } from "./race-records";
import { raceMarkKind, sortRaceRecords, type RaceRecord, type RaceRecords } from "@/lib/race-records";
import { todayInTokyo } from "@/lib/format";
import { senseDayLabel, type SenseNote, type SenseNoteStats } from "@/lib/sense-notes";

export function SenseNoteCalendar({ notes, token, month, onMonthChange, loading, failed, onRetry, onDeleted, records, selectedDay: controlledDay, onDayChange, onRecordsChanged, recordsLoading = false, recordsFailed = false, onRecordsRetry }: {
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
  const shared = records !== undefined;
  const title = shared ? "感覚と記録のカレンダー" : "感覚メモの履歴";
  const today = todayInTokyo();
  const [year, monthNumber] = month.split("-").map(Number);
  const offset = new Date(Date.UTC(year, monthNumber - 1, 1)).getUTCDay();
  const dayCount = new Date(Date.UTC(year, monthNumber, 0)).getUTCDate();
  const marked = new Set(notes.map((note) => note.day));
  const recorded = new Set(records?.map((record) => record.raced_on));
  const selectedRecords = sortRaceRecords(records?.filter((record) => record.raced_on === selectedDay) ?? []);
  const selectedNotes = notes.filter((note) => note.day === selectedDay);

  function moveMonth(delta: number) {
    const date = new Date(Date.UTC(year, monthNumber - 1 + delta, 1));
    setSelectedDay(null);
    onMonthChange(date.toISOString().slice(0, 7));
  }

  return (
    <section aria-label={title} className="space-y-4 rounded-2xl border border-white/20 glass bg-white/10 p-4">
      <h2 className="text-base font-bold">{title}</h2>
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
          const hasContent = written || hasRecord;
          return (
            <button
              key={day}
              type="button"
              disabled={!hasContent}
              aria-label={`${year}年${monthNumber}月${number}日${written ? "、感覚メモあり" : "、感覚メモなし"}${shared ? hasRecord ? "、記録あり" : "、記録なし" : ""}`}
              aria-pressed={selectedDay === day}
              aria-current={day === today ? "date" : undefined}
              onClick={() => setSelectedDay(day)}
              className={`flex min-h-12 min-w-0 flex-col items-center justify-center gap-1 rounded-xl border text-sm tabular-nums transition focus-visible:outline-2 focus-visible:outline-sky-300 ${selectedDay === day ? "border-sky-300 bg-sky-400/25 text-white" : hasContent ? "border-sky-300/30 bg-sky-400/10 text-sky-100 hover:bg-sky-400/20" : "border-transparent text-slate-400"} ${day === today ? "ring-1 ring-inset ring-white/40" : ""}`}
            >
              <span>{number}</span>
              <span className="flex gap-1" aria-hidden>
                <span className={`size-1.5 rounded-full ${written ? "bg-sky-300" : "bg-transparent"}`} />
                {shared ? <span className={`size-1.5 rounded-full ${hasRecord ? "bg-amber-300" : "bg-transparent"}`} /> : null}
              </span>
            </button>
          );
        })}
      </div>
      <p className="flex items-center gap-2 text-xs text-slate-300"><span aria-hidden className="size-1.5 rounded-full bg-sky-300" />書いた日をタップすると詳細が開きます</p>
      {shared ? <p className="flex flex-wrap gap-x-4 gap-y-1 text-xs text-slate-300"><span className="flex items-center gap-1"><span aria-hidden className="size-1.5 rounded-full bg-sky-300" />感覚メモ</span><span className="flex items-center gap-1"><span aria-hidden className="size-1.5 rounded-full bg-amber-300" />大会記録</span></p> : null}
      {recordsLoading ? <p role="status" className="text-sm text-slate-300">記録を読み込み中...</p> : recordsFailed ? <div role="alert"><p className="text-sm text-red-300">大会記録を読み込めませんでした。</p><Button variant="secondary" onClick={onRecordsRetry}>記録を再読み込み</Button></div> : null}
      {loading ? <p role="status" className="text-sm text-slate-300">この月のメモを読み込み中...</p> : failed ? <div role="alert" className="space-y-2"><p className="text-sm text-red-300">過去のメモを読み込めませんでした。</p><Button variant="secondary" onClick={onRetry}>再読み込み</Button></div> : !notes.some((note) => note.day.startsWith(month)) && !records?.some((record) => record.raced_on.startsWith(month)) && !recordsLoading && !recordsFailed ? <p className="text-sm text-slate-300">{shared ? "この月の感覚メモ・大会記録はありません。" : "この月の感覚メモはありません。"}</p> : null}
      {selectedDay && (selectedNotes.length > 0 || selectedRecords.length > 0) ? (
        <section aria-label={`${senseDayLabel(selectedDay)}の${shared ? "感覚メモと大会記録" : "感覚メモ"}`} className="space-y-2 border-t border-white/15 pt-3">
          <div className="flex items-center justify-between gap-2">
            <h3 className="text-sm font-semibold">{senseDayLabel(selectedDay)}</h3>
            <button type="button" aria-label="詳細を閉じる" onClick={() => setSelectedDay(null)} className="flex size-10 items-center justify-center rounded-full hover:bg-white/10"><X className="size-4" aria-hidden /></button>
          </div>
          {selectedRecords.length ? <h4 className="text-sm font-semibold text-amber-200">大会記録</h4> : null}
          {selectedRecords.map((record) => <RaceRecordRow key={record.id} record={record} kind={raceMarkKind(record.event)} token={token} onChanged={(next) => onRecordsChanged?.(next)} />)}
          {shared && selectedNotes.length ? <h4 className="text-sm font-semibold text-sky-200">感覚メモ</h4> : null}
          {selectedNotes.map((note) => <SenseNoteItem key={note.id} note={note} token={token} onDeleted={onDeleted} />)}
        </section>
      ) : null}
    </section>
  );
}
