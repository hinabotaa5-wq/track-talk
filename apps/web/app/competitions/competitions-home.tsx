"use client";

import { useEffect, useState, type ReactNode } from "react";
import Link from "next/link";
import { ChevronRight, Globe, MapPin, MessageCircle } from "lucide-react";
import { motion, MotionConfig } from "motion/react";
import { useSearchParams } from "next/navigation";
import { EmptyState, ErrorState } from "@/components/empty-state";
import { NowRunning } from "@/components/meet-now-running";
import { CompetitionLoadingFrame } from "@/components/loading-frame";
import { loadCompetitions, readCompetitions } from "@/lib/competition-list";
import { regionalCompetitionsPath } from "@/lib/competition-prefecture";
import { loadDomesticMajors, peekDomesticMajors, undecidedDomesticMeets, type DomesticMeet } from "@/lib/domestic-majors";
import { fetchTodayMeetThreads, meetThreadForCompetition, meetThreadForMajor } from "@/lib/meet-threads";
import { INTERNATIONAL } from "@/lib/prefectures";
import { isOngoing, isOver, seasonSchedule, type MonthGroup, type Scheduled } from "@/lib/season-schedule";
import { todayInTokyo } from "@/lib/format";
import { useNow } from "@/lib/use-now";
import type { Competition, DomesticMajor, MeetThread } from "@/lib/types";
import { competitionDates, destination } from "./regional/regional-view";

type View = "domestic" | "international";

const views: { id: View; label: string }[] = [
  { id: "domestic", label: "国内主要大会" },
  { id: "international", label: "国際大会" },
];

const entries = [
  { href: regionalCompetitionsPath, label: "都道府県の大会", icon: MapPin },
  { href: "/competitions/major-hosts", label: "今後の国際大会", icon: Globe },
];

export function CompetitionsHome({ initialMajors = null }: { initialMajors?: DomesticMajor[] | null }) {
  const sp = useSearchParams();
  const view: View = sp.get("view") === "international" ? "international" : "domestic";
  // 一度開いた一覧は消さずに隠しておき、行き来しても読み込み直さない。
  const [opened, setOpened] = useState<Set<View>>(() => new Set([view]));
  // 開催中の大会の実況スレッド。ホームの「今日開催中」と同じ API なので、開催中の判定もホームとそろう。
  // 取れなければ空のままで、どの大会もふだんどおり公式サイトへ飛ぶ。
  const [meets, setMeets] = useState<MeetThread[]>([]);

  useEffect(() => {
    let active = true;
    fetchTodayMeetThreads().then((next) => {
      if (active) setMeets(next);
    });
    return () => {
      active = false;
    };
  }, []);

  // router.replace だとサーバーまで取りに行って「読み込み中」が挟まるので、URL だけ書き換えてその場で切り替える。
  function select(next: View) {
    if (next === view) return;
    setOpened((prev) => (prev.has(next) ? prev : new Set(prev).add(next)));
    window.history.replaceState(null, "", next === "domestic" ? "/competitions" : "/competitions?view=international");
  }

  return (
    <div className="space-y-5">
      {/* 国内主要大会の一覧が主役。都道府県の大会と今後の国際大会は、いちばん上から入れるようにする。 */}
      <nav aria-label="ほかの大会" className="grid grid-cols-2 gap-2 md:max-w-md">
        {entries.map((entry) => {
          const Icon = entry.icon;
          return (
            <Link
              key={entry.href}
              href={entry.href}
              className="flex h-12 items-center justify-center gap-1.5 rounded-xl border border-white/25 glass bg-white/12 px-2 text-sm font-semibold text-slate-100 shadow-xs transition hover:bg-white/16 hover:text-white active:scale-95"
            >
              <Icon className="size-4 shrink-0 text-blue-300" aria-hidden />
              <span className="truncate">{entry.label}</span>
              <ChevronRight className="size-4 shrink-0 text-slate-400" aria-hidden />
            </Link>
          );
        })}
      </nav>
      <MotionConfig reducedMotion="user">
        <div role="group" aria-label="表示する大会" className="grid grid-cols-2 gap-1 rounded-full border border-white/25 glass bg-white/10 p-1 md:max-w-sm">
          {views.map((item) => {
            const active = item.id === view;
            return (
              <button
                key={item.id}
                type="button"
                aria-pressed={active}
                onClick={() => select(item.id)}
                className={`relative h-10 rounded-full text-sm font-semibold transition-colors duration-200 active:scale-95 ${
                  active ? "text-white" : "text-slate-200 hover:text-white"
                }`}
              >
                {active ? (
                  <motion.span
                    layoutId="competition-view-pill"
                    className="absolute inset-0 rounded-full bg-white/25"
                    transition={{ type: "spring", stiffness: 500, damping: 40 }}
                    aria-hidden
                  />
                ) : null}
                <span className="relative">{item.label}</span>
              </button>
            );
          })}
        </div>
        {views.map((item) =>
          opened.has(item.id) || item.id === view ? (
            <motion.div
              key={item.id}
              hidden={item.id !== view}
              initial={{ opacity: 0, y: 8 }}
              animate={item.id === view ? { opacity: 1, y: 0 } : { opacity: 0, y: 8 }}
              transition={{ duration: 0.22, ease: "easeOut" }}
            >
              {item.id === "domestic" ? (
                <DomesticSchedule initial={initialMajors} meets={meets} />
              ) : (
                <InternationalSchedule meets={meets} />
              )}
            </motion.div>
          ) : null,
        )}
      </MotionConfig>
    </div>
  );
}

function DomesticSchedule({ initial, meets }: { initial: DomesticMajor[] | null; meets: MeetThread[] }) {
  const [majors, setMajors] = useState<DomesticMajor[] | null>(() => peekDomesticMajors() ?? initial);
  const [error, setError] = useState("");

  useEffect(() => {
    let active = true;
    loadDomesticMajors()
      .then((next) => {
        if (!active) return;
        setMajors(next);
        setError("");
      })
      .catch(() => {
        if (active) setError("国内主要大会を取得できませんでした。");
      });
    return () => {
      active = false;
    };
  }, []);

  if (error) return <ErrorState message={error} />;
  if (!majors) return <CompetitionLoadingFrame />;
  return (
    <Schedule
      items={majors}
      itemKey={(major) => `${major.slug}:${major.year}`}
      renderCard={(major, today) => <DomesticCard major={major} today={today} thread={meetThreadForMajor(meets, major)} />}
      undecided={undecidedDomesticMeets(majors)}
      emptyBody="今シーズンの国内主要大会はまだ発表されていません。"
    />
  );
}

function InternationalSchedule({ meets }: { meets: MeetThread[] }) {
  const cached = readCompetitions(INTERNATIONAL, 0);
  const [entry, setEntry] = useState(cached);

  useEffect(() => {
    let active = true;
    loadCompetitions(INTERNATIONAL, 0).then((next) => {
      if (active) setEntry(next);
    });
    return () => {
      active = false;
    };
  }, []);

  if (!entry) return <CompetitionLoadingFrame />;
  if (entry.error) return <ErrorState message={entry.error} />;
  return (
    <Schedule
      items={entry.items}
      itemKey={(c) => c.id}
      renderCard={(c, today) => <InternationalCard competition={c} today={today} thread={meetThreadForCompetition(meets, c)} />}
      emptyBody="今シーズンの国際大会はまだ取り込まれていません。"
    />
  );
}

function Schedule<T extends Scheduled>({
  items,
  itemKey,
  renderCard,
  undecided = [],
  emptyBody,
}: {
  items: T[];
  itemKey: (item: T) => string;
  renderCard: (item: T, today: string) => ReactNode;
  undecided?: DomesticMeet[];
  emptyBody: string;
}) {
  // ページは CDN にキャッシュされるので、「今日」は表示のあとに決める。それまでは日付に依存しない形
  // （すべて開催予定、バッジなし）で出し、キャッシュされた前日の振り分けを見せない。
  const now = useNow();
  const today = now === null ? "0000-01-01" : todayInTokyo(new Date(now));
  const { upcoming, ended } = seasonSchedule(items, today);
  if (items.length === 0 && undecided.length === 0) {
    return <EmptyState title="大会がありません" body={emptyBody} />;
  }

  const months = (groups: MonthGroup<T>[]) =>
    groups.map((group) => (
      <section key={group.key} className="space-y-3">
        <h3 className="text-base font-semibold text-slate-200">{group.label}</h3>
        <div className="grid gap-3 md:grid-cols-2">
          {group.items.map((item) => (
            <div key={itemKey(item)}>{renderCard(item, today)}</div>
          ))}
        </div>
      </section>
    ));

  return (
    <div className="space-y-8">
      <section className="space-y-4">
        <h2 className="text-xl font-semibold text-slate-100">開催予定</h2>
        {upcoming.length === 0 && undecided.length === 0 ? (
          <p className="text-sm text-slate-200">今シーズンの開催予定の大会はありません。</p>
        ) : null}
        {months(upcoming)}
        {undecided.length > 0 ? (
          <section className="space-y-3">
            <h3 className="text-base font-semibold text-slate-200">日程未定</h3>
            <div className="grid gap-3 md:grid-cols-2">
              {undecided.map((meet) => (
                <div key={meet.slug} className={`${cardBase} opacity-70`}>
                  <p className="text-xs text-slate-200">日程未定</p>
                  <h4 className="text-lg font-semibold text-slate-100">{meet.label}</h4>
                </div>
              ))}
            </div>
          </section>
        ) : null}
      </section>
      {ended.length > 0 ? (
        <section className="space-y-4">
          <h2 className="text-xl font-semibold text-slate-100">終了済み</h2>
          {months(ended)}
        </section>
      ) : null}
    </div>
  );
}

const cardBase = "block h-full rounded-xl border border-white/20 glass bg-white/10 p-4";

function StateBadge({ item, today }: { item: Scheduled; today: string }) {
  if (isOver(item, today)) return <span className="ml-2 rounded-full bg-white/15 px-2 py-0.5 text-slate-100">終了</span>;
  if (isOngoing(item, today)) return <span className="ml-2 rounded-full bg-blue-500/30 px-2 py-0.5 text-blue-100">開催中</span>;
  return null;
}

// ふだんは公式サイト(要項)へ飛ぶ。開催中で実況スレッドがある大会だけ、カードのタップで実況スレッドへ入り、公式サイトは下の小さなリンクに残す。
function ScheduleCard({
  href,
  linkLabel,
  over,
  thread,
  children,
}: {
  href: string;
  linkLabel: string;
  over: boolean;
  thread?: MeetThread;
  children: ReactNode;
}) {
  if (thread) {
    return (
      <div className={`${cardBase} border-amber-300/60`}>
        <Link href={thread.href} className="block transition active:scale-[0.98]">
          {children}
          <span className="mt-2 flex items-center justify-between gap-2 rounded-lg bg-amber-400/20 px-3 py-2 text-sm font-semibold text-amber-100 transition hover:bg-amber-400/30">
            <span className="min-w-0">
              <span className="block">実況スレッドへ</span>
              <NowRunning events={thread.now_running} className="block truncate text-xs font-semibold text-emerald-200" />
            </span>
            <span className="flex shrink-0 items-center gap-1 text-xs text-amber-100/80">
              <MessageCircle className="size-3.5" aria-hidden />
              {thread.comment_count}
              <ChevronRight className="size-4" aria-hidden />
            </span>
          </span>
        </Link>
        {href && linkLabel ? (
          <a href={href} target="_blank" rel="noopener noreferrer" className="mt-2 inline-block text-sm text-blue-400 hover:underline">
            {linkLabel}
          </a>
        ) : null}
      </div>
    );
  }

  const label = linkLabel ? <p className="text-sm text-blue-400">{linkLabel}</p> : null;
  if (!href) {
    return (
      <div className={cardBase}>
        {children}
        {label}
      </div>
    );
  }
  return (
    <a
      href={href}
      target="_blank"
      rel="noopener noreferrer"
      className={`${cardBase} transition hover:border-blue-700/70 hover:bg-white/16 active:scale-95 ${over ? "opacity-80" : ""}`}
    >
      {children}
      {label}
    </a>
  );
}

function DomesticCard({ major, today, thread }: { major: DomesticMajor; today: string; thread?: MeetThread }) {
  return (
    <ScheduleCard href={major.site_url} linkLabel={major.site_url ? "公式サイト" : ""} over={isOver(major, today)} thread={thread}>
      <p className="text-xs text-slate-200">
        {major.schedule_label}
        <StateBadge item={major} today={today} />
      </p>
      <h4 className="text-lg font-semibold text-slate-100">{major.name}</h4>
      {major.venue ? <p className="mt-1 text-sm text-slate-200">{major.venue}</p> : null}
    </ScheduleCard>
  );
}

function InternationalCard({ competition, today, thread }: { competition: Competition; today: string; thread?: MeetThread }) {
  const link = destination(competition, INTERNATIONAL);
  return (
    <ScheduleCard href={link.href} linkLabel={link.label} over={isOver(competition, today)} thread={thread}>
      <p className="text-xs text-slate-200">
        {competitionDates(competition.starts_on, competition.ends_on)}
        <StateBadge item={competition} today={today} />
      </p>
      <h4 className="text-lg font-semibold text-slate-100">{competition.name}</h4>
      {competition.location ? <p className="mt-1 text-sm text-slate-200">{competition.location}</p> : null}
    </ScheduleCard>
  );
}
