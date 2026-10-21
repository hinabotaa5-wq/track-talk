"use client";

import { useRouter } from "next/navigation";
import { useEffect, useState } from "react";
import { useAuth } from "@/components/auth-provider";
import { EmptyState, ErrorState } from "@/components/empty-state";
import { RaceRecordsPanel } from "@/components/race-records";
import { SenseNoteJar } from "@/components/sense-note-jar";
import { SenseNoteComposer, SenseNoteItem, SenseNoteStatsRow } from "@/components/sense-notes";
import { SenseSummaryCard } from "@/components/sense-summary";
import { Button } from "@/components/ui/button";
import { ViewSwitch } from "@/components/view-switch";
import { apiFetch } from "@/lib/api";
import { senseSummaryEnabled } from "@/lib/features";
import { groupSenseNotesByDay, senseDayLabel, sortSenseNotes, type SenseNote, type SenseNoteStats } from "@/lib/sense-notes";

type Page = { notes: SenseNote[]; next_cursor: string | null; stats: SenseNoteStats };

export default function MySenseNotesPage() {
  const { session, token, loading } = useAuth();
  const router = useRouter();
  const [notes, setNotes] = useState<SenseNote[] | null>(null);
  const [stats, setStats] = useState<SenseNoteStats | null>(null);
  const [cursor, setCursor] = useState<string | null>(null);
  const [failed, setFailed] = useState(false);
  const [loadingMore, setLoadingMore] = useState(false);
  const [dropped, setDropped] = useState(false);
  // 感覚メモと、競技会の記録(グラフ)を切り替える。
  const [tab, setTab] = useState<"sense" | "records">("sense");

  useEffect(() => {
    if (!loading && !session) router.replace("/login?next=/memo");
  }, [loading, session, router]);

  // 月1回の記録のお知らせ(プッシュ通知)は /memo?tab=records を開く。
  useEffect(() => {
    // eslint-disable-next-line react-hooks/set-state-in-effect
    if (new URLSearchParams(window.location.search).get("tab") === "records") setTab("records");
  }, []);

  useEffect(() => {
    if (!token) return;
    const controller = new AbortController();
    apiFetch<Page>("/v1/me/sense-notes", { token, signal: controller.signal })
      .then((d) => {
        setNotes(d.notes ?? []);
        setStats(d.stats);
        setCursor(d.next_cursor);
      })
      .catch(() => {
        if (!controller.signal.aborted) setFailed(true);
      });
    return () => controller.abort();
  }, [token]);

  async function loadMore() {
    if (!cursor || loadingMore) return;
    setLoadingMore(true);
    try {
      const d = await apiFetch<Page>(`/v1/me/sense-notes?cursor=${encodeURIComponent(cursor)}`, { token });
      setNotes((list) => [...(list ?? []), ...(d.notes ?? [])]);
      setCursor(d.next_cursor);
    } catch {
      alert("読み込めませんでした");
    } finally {
      setLoadingMore(false);
    }
  }

  if (loading || !session) return <p className="text-sm text-slate-200">読み込み中...</p>;

  return (
    <div className="mx-auto max-w-2xl space-y-4">
      <h1 className="sr-only">メモ</h1>
      <ViewSwitch
        label="感覚と記録の切り替え"
        layoutId="memo-view-pill"
        views={[
          { id: "sense", label: "感覚" },
          { id: "records", label: "記録" },
        ]}
        current={tab}
        onSelect={setTab}
      />

      {tab === "records" ? (
        <RaceRecordsPanel token={token} />
      ) : failed ? (
        <ErrorState message="感覚メモの取得に失敗しました。APIの起動を確認してください。" />
      ) : notes === null || stats === null ? (
        <p className="text-sm text-slate-200">読み込み中...</p>
      ) : (
        <>
          <section className="space-y-3 rounded-2xl border border-white/20 glass bg-white/10 p-4">
            <div className="flex items-center gap-4">
              <SenseNoteJar total={stats.total} dropLatest={dropped} className="h-32 w-24" />
              <div className="min-w-0 flex-1">
                <SenseNoteStatsRow stats={stats} />
              </div>
            </div>
            <SenseNoteComposer
              token={token}
              onCreated={(note, next) => {
                setNotes((list) => sortSenseNotes([note, ...(list ?? [])]));
                setStats(next);
                setDropped(true);
              }}
            />
          </section>

          {senseSummaryEnabled && notes.length > 0 ? <SenseSummaryCard token={token} total={stats.total} /> : null}

          {notes.length === 0 ? (
            <EmptyState title="まだ感覚メモがありません" body="今日の練習で「うまくいった」「なんか違った」と感じたことを、ひとことから書いてみよう。" />
          ) : (
            <div className="space-y-5">
              {groupSenseNotesByDay(notes).map((group) => (
                <section key={group.day} className="space-y-2">
                  <h2 className="text-sm font-semibold text-slate-200">{senseDayLabel(group.day)}</h2>
                  <ul className="space-y-2">
                    {group.notes.map((note) => (
                      <li key={note.id}>
                        <SenseNoteItem
                          note={note}
                          token={token}
                          onUpdated={(updated) => setNotes((list) => list?.map((n) => (n.id === updated.id ? updated : n)) ?? list)}
                          onDeleted={(id, next) => {
                            setNotes((list) => list?.filter((n) => n.id !== id) ?? list);
                            setStats(next);
                            setDropped(false);
                          }}
                        />
                      </li>
                    ))}
                  </ul>
                </section>
              ))}
              {cursor ? (
                <div className="flex justify-center">
                  <Button variant="secondary" onClick={loadMore} disabled={loadingMore}>
                    {loadingMore ? "読み込み中..." : "もっと見る"}
                  </Button>
                </div>
              ) : null}
            </div>
          )}
        </>
      )}
    </div>
  );
}
