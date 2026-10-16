"use client";

import Link from "next/link";
import { ChevronRight } from "lucide-react";
import { useEffect, useState } from "react";
import { SenseNoteJar } from "./sense-note-jar";
import { SenseNoteComposer, SenseNoteStatsRow } from "./sense-notes";
import { apiFetch } from "@/lib/api";
import { emptySenseNoteStats, senseDayLabel, type SenseNote, type SenseNoteStats } from "@/lib/sense-notes";

// マイページの感覚メモ。ビンと数字で「たまっている」ことを見せ、その場で今日の感覚を書ける。
export function SenseNotesCard({ token }: { token: string | null }) {
  const [stats, setStats] = useState<SenseNoteStats | null>(null);
  const [recent, setRecent] = useState<SenseNote[]>([]);
  const [dropped, setDropped] = useState(false);

  useEffect(() => {
    if (!token) return;
    const controller = new AbortController();
    apiFetch<{ notes: SenseNote[]; stats: SenseNoteStats }>("/v1/me/sense-notes", { token, signal: controller.signal })
      .then((d) => {
        setStats(d.stats);
        setRecent((d.notes ?? []).slice(0, 2));
      })
      .catch(() => {
        // 読めなくても書けるようにしておく。数字は0から数える。
        if (!controller.signal.aborted) setStats(emptySenseNoteStats);
      });
    return () => controller.abort();
  }, [token]);

  const current = stats ?? emptySenseNoteStats;

  return (
    <section aria-labelledby="sense-notes-title" className="space-y-3 rounded-2xl border border-white/20 glass bg-white/10 p-4">
      <div className="flex items-center gap-4">
        <SenseNoteJar total={current.total} dropLatest={dropped} className="h-28 w-21" />
        <div className="min-w-0 flex-1 space-y-2">
          <div>
            <h2 id="sense-notes-title" className="text-lg font-bold">
              感覚メモ
            </h2>
            <p className="text-xs text-slate-300">練習メニューではなく、体を動かして感じたことを。</p>
          </div>
          <SenseNoteStatsRow stats={current} />
        </div>
      </div>
      <SenseNoteComposer
        token={token}
        onCreated={(note, next) => {
          setStats(next);
          setRecent((list) => [note, ...list].slice(0, 2));
          setDropped(true);
        }}
      />
      {recent.length > 0 ? (
        <ul className="space-y-1.5 border-t border-white/15 pt-3">
          {recent.map((note) => (
            <li key={note.id} className="flex gap-2 text-sm">
              <span className="shrink-0 text-xs leading-5 text-slate-300">{senseDayLabel(note.day)}</span>
              <span className="min-w-0 flex-1 truncate text-slate-100">{note.body}</span>
            </li>
          ))}
        </ul>
      ) : null}
      <Link href="/me/sense" className="flex min-h-11 items-center justify-between rounded-xl px-1 text-sm font-semibold text-sky-200 hover:underline">
        これまでの感覚メモを見る
        <ChevronRight className="size-4" aria-hidden />
      </Link>
    </section>
  );
}
