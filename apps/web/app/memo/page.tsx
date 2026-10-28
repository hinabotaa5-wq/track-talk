"use client";

import { useRouter } from "next/navigation";
import { useEffect, useState, type SyntheticEvent } from "react";
import { useAuth } from "@/components/auth-provider";
import { ErrorState } from "@/components/empty-state";
import { RaceRecordsPanel } from "@/components/race-records";
import { SenseNoteJar } from "@/components/sense-note-jar";
import { SenseNoteComposer, SenseNoteStatsRow } from "@/components/sense-notes";
import { SenseSummaryCard, SenseSummaryResult, useSenseSummaries } from "@/components/sense-summary";
import { SenseNoteCalendar } from "@/components/sense-note-calendar";
import { TargetMeetCountdown } from "@/components/target-meet-countdown";
import type { RaceRecords } from "@/lib/race-records";
import { todayInTokyo } from "@/lib/format";
import { ViewSwitch } from "@/components/view-switch";
import { apiFetch } from "@/lib/api";
import { senseSummaryEnabled } from "@/lib/features";
import { sortSenseNotes, type SenseNote, type SenseNoteStats } from "@/lib/sense-notes";

type Page = { notes: SenseNote[]; next_cursor: string | null; stats: SenseNoteStats };

const guestStats: SenseNoteStats = { total: 0, days: 0, streak: 0, today: 0 };
const loginPath = "/login?next=/memo";

export default function MySenseNotesPage() {
  const { session, token, loading } = useAuth();
  const router = useRouter();
  const [notes, setNotes] = useState<SenseNote[] | null>(null);
  const [stats, setStats] = useState<SenseNoteStats | null>(null);
  const [cursor, setCursor] = useState<string | null>(null);
  const [notesRetry, setNotesRetry] = useState(0);
  const [failed, setFailed] = useState(false);
  const [historyFailed, setHistoryFailed] = useState(false);
  const [month, setMonth] = useState(() => todayInTokyo().slice(0, 7));
  const [raceData, setRaceData] = useState<RaceRecords | null>(null);
  const [raceFailed, setRaceFailed] = useState(false);
  const [raceRetry, setRaceRetry] = useState(0);
  const [selectedDay, setSelectedDay] = useState<string | null>(null);
  const [dropped, setDropped] = useState(false);
  // 感覚メモと、競技会の記録(グラフ)を切り替える。
  const [tab, setTab] = useState<"sense" | "records">("sense");

  // ログインしていなくても中身は見られる。書こうとしたり、ボタンを押したりしたらログインへ案内する。
  const guest = !loading && !session;
  const summaryState = useSenseSummaries(senseSummaryEnabled && session ? token : null, stats?.total ?? 0, month);
  function toLogin(event: SyntheticEvent) {
    event.preventDefault();
    event.stopPropagation();
    if (event.target instanceof HTMLElement) event.target.blur();
    router.push(loginPath);
  }

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
        if (controller.signal.aborted) return;
        setFailed(false);
        setNotes(d.notes ?? []);
        setStats(d.stats);
        setCursor(d.next_cursor);
      })
      .catch(() => {
        if (!controller.signal.aborted) setFailed(true);
      });
    return () => controller.abort();
  }, [token, notesRetry]);

  useEffect(() => {
    if (!token) return;
    const controller = new AbortController();
    apiFetch<RaceRecords>("/v1/me/race-records", { token, signal: controller.signal })
      .then((next) => { if (!controller.signal.aborted) { setRaceData(next); setRaceFailed(false); } })
      .catch(() => { if (!controller.signal.aborted) setRaceFailed(true); });
    return () => controller.abort();
  }, [token, raceRetry]);

  // 月初より古いメモに届くまでページを読み、月内のマークが欠けないようにする。
  useEffect(() => {
    const oldest = notes?.[notes.length - 1]?.day;
    if (!token || !cursor || !oldest || oldest < `${month}-01` || historyFailed) return;
    const controller = new AbortController();
    apiFetch<Page>(`/v1/me/sense-notes?cursor=${encodeURIComponent(cursor)}`, { token, signal: controller.signal })
      .then((d) => {
        setNotes((list) => sortSenseNotes([...new Map([...(list ?? []), ...(d.notes ?? [])].map((note) => [note.id, note])).values()]));
        setCursor(d.next_cursor);
      })
      .catch(() => {
        if (!controller.signal.aborted) setHistoryFailed(true);
      });
    return () => controller.abort();
  }, [token, cursor, notes, month, historyFailed]);

  if (loading) return <p className="text-sm text-slate-200">読み込み中...</p>;
  const shownNotes = guest ? [] : notes;
  const shownStats = guest ? guestStats : stats;
  const needsOlderNotes = Boolean(cursor && shownNotes?.length && shownNotes[shownNotes.length - 1].day >= `${month}-01`);

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

      <div
        className="space-y-4"
        onFocusCapture={guest ? toLogin : undefined}
        onClickCapture={guest ? toLogin : undefined}
        onSubmitCapture={guest ? toLogin : undefined}
      >
        {guest ? (
          <p className="rounded-xl border border-sky-300/30 bg-sky-950/40 p-3 text-sm text-sky-100">
            ログインすると、感覚や記録をためて、あとからふりかえれます。
          </p>
        ) : null}
        {tab === "records" ? (
          raceFailed ? <ErrorState message="記録の取得に失敗しました。下のカレンダーから再読み込みできます。" /> :
          <RaceRecordsPanel token={guest ? null : token} data={raceData} onRecordsChanged={setRaceData} selectedDay={selectedDay} />
        ) : failed ? (
          <ErrorState message="感覚メモの取得に失敗しました。時間をおいて、もう一度開いてください。" />
        ) : shownNotes === null || shownStats === null ? (
          <p className="text-sm text-slate-200">読み込み中...</p>
        ) : (
          <>
            <section className="space-y-3 rounded-2xl border border-white/20 glass bg-white/10 p-4">
              <div className="flex items-center gap-4">
                <SenseNoteJar total={shownStats.total} dropLatest={dropped} className="h-32 w-24" />
                <div className="min-w-0 flex-1 space-y-3">
                  <TargetMeetCountdown token={guest ? null : token} />
                  {senseSummaryEnabled && !guest ? <SenseSummaryCard state={summaryState} compact /> : null}
                  <SenseNoteStatsRow stats={shownStats} />
                </div>
              </div>
              <SenseNoteComposer
                token={token}
                writtenToday={shownStats.today > 0}
                streak={shownStats.streak}
                onCreated={(note, next) => {
                  setNotes((list) => sortSenseNotes([note, ...(list ?? [])]));
                  setStats(next);
                  setDropped(true);
                }}
              />
            </section>
          </>
        )}
        <SenseSummaryResult state={summaryState}>
            <SenseNoteCalendar
              notes={shownNotes ?? []}
              summaries={summaryState.summaries}
              summariesLoading={senseSummaryEnabled && !guest && (!summaryState.loaded || summaryState.loadingMore) && !summaryState.error}
              summariesFailed={senseSummaryEnabled && !guest && Boolean(summaryState.error)}
              onSummariesRetry={summaryState.retry}
              token={token}
              records={guest ? [] : raceData?.records ?? []}
              selectedDay={selectedDay}
              onDayChange={setSelectedDay}
              onRecordsChanged={setRaceData}
              recordsLoading={!guest && !raceData && !raceFailed}
              recordsFailed={!guest && raceFailed}
              onRecordsRetry={() => { setRaceFailed(false); setRaceRetry((value) => value + 1); }}
              month={month}
              onMonthChange={setMonth}
              loading={!guest && !failed && (shownNotes === null || (needsOlderNotes && !historyFailed))}
              failed={failed || (needsOlderNotes && historyFailed)}
              onRetry={() => { setHistoryFailed(false); if (failed) { setFailed(false); setNotesRetry((value) => value + 1); } }}
              onDeleted={(id, next) => {
                setNotes((list) => list?.filter((n) => n.id !== id) ?? list);
                setStats(next);
                setDropped(false);
              }}
            />
        </SenseSummaryResult>

      </div>
    </div>
  );
}
