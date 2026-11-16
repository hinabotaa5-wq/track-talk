"use client";
import { L, weekdayLabel } from "@/lib/i18n";

import { useEffect, useRef, useState } from "react";
import { Check, ChevronLeft, ChevronRight, X } from "lucide-react";
import { Button } from "./ui/button";
import { apiFetch } from "@/lib/api";
import { todayInTokyo } from "@/lib/format";
import { senseDayLabel, type ContractSenseNote, type SenseNoteSharingResponse } from "@/lib/sense-notes";

// 長押しでプレビューを開くまでの時間。
const previewLongPressMs = 500;

// starts_at の Asia/Tokyo 日付から1ヶ月前の月の "YYYY-MM" を求める。存在しない同日は考えず、月だけ返す。
function windowStartMonth(startsAt: string | null): string | null {
  const parsed = Date.parse(startsAt ?? "");
  if (Number.isNaN(parsed)) return null;
  const jst = new Date(parsed + 9 * 3600 * 1000);
  let year = jst.getUTCFullYear();
  let month = jst.getUTCMonth() + 1 - 1;
  if (month < 1) {
    month = 12;
    year -= 1;
  }
  return `${year}-${String(month).padStart(2, "0")}`;
}

export function SenseNoteSharing({ contractId, token, isCoach, startsAt }: {
  contractId: string;
  token: string | null;
  isCoach: boolean;
  startsAt: string | null;
}) {
  const [notes, setNotes] = useState<ContractSenseNote[] | null>(null);
  const [viewClosed, setViewClosed] = useState(false);
  const [failed, setFailed] = useState(false);
  const [attempt, setAttempt] = useState(0);
  const [error, setError] = useState("");
  const [previewDay, setPreviewDay] = useState<string | null>(null);
  const todayMonth = todayInTokyo().slice(0, 7);
  const startMonth = windowStartMonth(startsAt);
  const minMonth = startMonth && startMonth <= todayMonth ? startMonth : todayMonth;
  const [month, setMonth] = useState(todayMonth);
  const pressTimer = useRef<number | null>(null);
  const pressPreviewed = useRef(false);
  const previewClose = useRef<HTMLButtonElement>(null);

  useEffect(() => {
    let cancelled = false;
    apiFetch<SenseNoteSharingResponse>(`/v1/coaching/contracts/${contractId}/sense-notes`, { token })
      .then((d) => {
        if (cancelled) return;
        setNotes(Array.isArray(d.notes) ? d.notes : []);
        setViewClosed(d.view_closed === true);
      })
      .catch(() => {
        if (!cancelled) setFailed(true);
      });
    return () => {
      cancelled = true;
    };
  }, [contractId, token, attempt]);

  useEffect(() => () => {
    if (pressTimer.current !== null) window.clearTimeout(pressTimer.current);
  }, []);

  useEffect(() => {
    if (previewDay) previewClose.current?.focus({ preventScroll: true });
  }, [previewDay]);

  function moveMonth(delta: number) {
    const [year, monthNumber] = month.split("-").map(Number);
    const date = new Date(Date.UTC(year, monthNumber - 1 + delta, 1));
    const next = date.toISOString().slice(0, 7);
    if (next < minMonth || next > todayMonth) return;
    setPreviewDay(null);
    setMonth(next);
  }

  async function toggleShare(note: ContractSenseNote) {
    const next = !note.shared;
    setNotes((prev) => (prev ?? []).map((n) => (n.id === note.id ? { ...n, shared: next } : n)));
    setError("");
    try {
      const d = await apiFetch<{ shared: boolean }>(
        `/v1/coaching/contracts/${contractId}/sense-notes/${note.id}/share`,
        { method: "POST", token, body: JSON.stringify({ shared: next }) },
      );
      setNotes((prev) => (prev ?? []).map((n) => (n.id === note.id ? { ...n, shared: d.shared } : n)));
    } catch {
      setNotes((prev) => (prev ?? []).map((n) => (n.id === note.id ? { ...n, shared: !next } : n)));
      setError(L("送信に失敗しました"));
    }
  }

  function startPress(day: string) {
    pressPreviewed.current = false;
    if (pressTimer.current !== null) window.clearTimeout(pressTimer.current);
    pressTimer.current = window.setTimeout(() => {
      pressTimer.current = null;
      pressPreviewed.current = true;
      setPreviewDay(day);
    }, previewLongPressMs);
  }

  function cancelPress() {
    if (pressTimer.current !== null) {
      window.clearTimeout(pressTimer.current);
      pressTimer.current = null;
    }
  }

  if (viewClosed) return null;

  const title = isCoach ? L("選手の感覚メモ") : L("感覚メモの共有");
  const [year, monthNumber] = month.split("-").map(Number);
  const offset = new Date(Date.UTC(year, monthNumber - 1, 1)).getUTCDay();
  const dayCount = new Date(Date.UTC(year, monthNumber, 0)).getUTCDate();
  const byDay = new Map<string, ContractSenseNote>();
  for (const note of notes ?? []) {
    if (!byDay.has(note.day)) byDay.set(note.day, note);
  }
  const monthNotes = (notes ?? []).filter((note) => note.day.startsWith(month));
  const preview = previewDay ? byDay.get(previewDay) ?? null : null;
  const today = todayInTokyo();

  return (
    <section aria-label={title} className="relative isolate rounded-2xl border border-white/20 glass bg-white/10 p-4">
      <div className="space-y-4">
        <h2 className="text-base font-bold">{title}</h2>
        {isCoach ? null : <p className="text-xs text-slate-300">{L("チェックを外すと、コーチはそのメモを見られなくなります")}</p>}
        <div className="flex items-center justify-between">
          <Button variant="secondary" aria-label={L("前の月")} disabled={month <= minMonth} onClick={() => moveMonth(-1)}><ChevronLeft className="size-5" aria-hidden /></Button>
          <h3 aria-live="polite" className="font-semibold tabular-nums">{L("{year}年{monthNumber}月", { year, monthNumber })}</h3>
          <Button variant="secondary" aria-label={L("次の月")} disabled={month >= todayMonth} onClick={() => moveMonth(1)}><ChevronRight className="size-5" aria-hidden /></Button>
        </div>
        {notes === null ? (
          failed ? (
            <div role="alert" className="space-y-2">
              <p className="text-sm text-red-300">{L("読み込みに失敗しました")}</p>
              <Button variant="secondary" onClick={() => { setFailed(false); setAttempt((n) => n + 1); }}>{L("再読み込み")}</Button>
            </div>
          ) : (
            <p role="status" className="text-sm text-slate-300">{L("読み込み中...")}</p>
          )
        ) : isCoach && notes.length === 0 ? (
          <p className="text-sm text-slate-300">{L("共有されている感覚メモはありません")}</p>
        ) : (
          <div className="grid grid-cols-7 gap-1 text-center">
            {Array.from({ length: 7 }, (_, index) => weekdayLabel(index)).map((day, index) => <span key={index} className={`py-2 text-xs ${index === 0 ? "text-rose-300" : index === 6 ? "text-sky-300" : "text-slate-300"}`}>{day}</span>)}
            {Array.from({ length: offset }, (_, index) => <span key={`blank-${index}`} aria-hidden />)}
            {Array.from({ length: dayCount }, (_, index) => {
              const number = index + 1;
              const day = `${month}-${String(number).padStart(2, "0")}`;
              const note = byDay.get(day) ?? null;
              if (isCoach) {
                return (
                  <button
                    key={day}
                    type="button"
                    disabled={!note}
                    aria-label={L("{year}年{monthNumber}月{number}日{v3}{v4}{v5}", { year, monthNumber, number, v3: note ? L("、感覚メモあり") : L("、感覚メモなし"), v4: "", v5: "" })}
                    aria-current={day === today ? "date" : undefined}
                    onClick={() => {
                      if (note) setPreviewDay(day);
                    }}
                    className={`flex min-h-12 min-w-0 flex-col items-center justify-center gap-1 rounded-xl border text-sm tabular-nums transition focus-visible:outline-2 focus-visible:outline-sky-300 ${note ? "border-sky-300/30 bg-sky-400/10 text-sky-100 hover:bg-sky-400/20" : "border-transparent text-slate-400"} ${day === today ? "ring-1 ring-inset ring-white/40" : ""}`}
                  >
                    <span>{number}</span>
                    <span className="flex h-1.5 items-center justify-center gap-1" aria-hidden>
                      {note ? <span className="size-1.5 rounded-full bg-sky-300" /> : null}
                    </span>
                  </button>
                );
              }
              return (
                <button
                  key={day}
                  type="button"
                  disabled={!note}
                  aria-pressed={note ? note.shared : undefined}
                  aria-label={L("{year}年{monthNumber}月{number}日{v3}{v4}{v5}", { year, monthNumber, number, v3: note ? L("、感覚メモあり") : L("、感覚メモなし"), v4: note ? (note.shared ? ` ${L("共有")}` : ` ${L("非表示")}`) : "", v5: "" })}
                  aria-current={day === today ? "date" : undefined}
                  onPointerDown={() => {
                    if (note) startPress(day);
                  }}
                  onPointerUp={cancelPress}
                  onPointerLeave={cancelPress}
                  onPointerCancel={cancelPress}
                  onClick={() => {
                    if (!note) return;
                    if (pressPreviewed.current) {
                      pressPreviewed.current = false;
                      return;
                    }
                    void toggleShare(note);
                  }}
                  className={`flex min-h-12 min-w-0 flex-col items-center justify-center gap-1 rounded-xl border text-sm tabular-nums transition focus-visible:outline-2 focus-visible:outline-sky-300 ${!note ? "border-transparent text-slate-400" : note.shared ? "border-sky-300 bg-sky-400/25 text-pure-white" : "border-white/10 bg-white/5 text-slate-400 opacity-70"} ${day === today ? "ring-1 ring-inset ring-white/40" : ""}`}
                >
                  <span>{number}</span>
                  <span className="flex h-1.5 items-center justify-center" aria-hidden>
                    {note && note.shared ? <Check className="size-3.5" /> : null}
                  </span>
                </button>
              );
            })}
          </div>
        )}
        {!isCoach && notes !== null && monthNotes.length === 0 && !failed ? <p className="text-sm text-slate-300">{L("この月の感覚メモはありません。")}</p> : null}
        {error ? <p role="alert" className="text-sm text-red-300">{error}</p> : null}
      </div>
      {previewDay && preview ? (
        <div className="absolute inset-0 z-20 flex items-center justify-center rounded-2xl bg-slate-950/60 px-2 backdrop-blur-sm" onClick={(event) => { if (event.target === event.currentTarget) setPreviewDay(null); }}>
          <section role="dialog" aria-label={L("{v0}の詳細", { v0: senseDayLabel(previewDay) })} onKeyDown={(event) => { if (event.key === "Escape") { event.stopPropagation(); setPreviewDay(null); } }} className="flex max-h-full w-full min-w-0 flex-col overflow-hidden rounded-xl border border-white/25 bg-slate-900 shadow-xl">
            <div className="flex shrink-0 items-center justify-between gap-2 border-b border-white/15 px-3 py-1">
              <h3 className="text-sm font-semibold">{senseDayLabel(previewDay)}</h3>
              <button type="button" ref={previewClose} aria-label={L("詳細を閉じる")} onClick={() => setPreviewDay(null)} className="flex size-10 items-center justify-center rounded-full hover:bg-white/10"><X className="size-4" aria-hidden /></button>
            </div>
            <div className="min-h-0 space-y-2 overflow-y-auto overscroll-contain p-3">
              {preview.event_label ? <p className="text-sm font-semibold text-sky-200">{preview.event_label}</p> : null}
              <p className="text-sm whitespace-pre-wrap text-slate-100">{preview.body}</p>
            </div>
          </section>
        </div>
      ) : null}
    </section>
  );
}
