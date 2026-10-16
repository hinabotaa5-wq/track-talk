"use client";

import { useEffect, useState, type ReactNode } from "react";
import Link from "next/link";
import { ChevronRight, Globe, MapPin } from "lucide-react";
import { useRouter, useSearchParams } from "next/navigation";
import { useAuth } from "@/components/auth-provider";
import { EmptyState, ErrorState } from "@/components/empty-state";
import { CompetitionLoadingFrame } from "@/components/loading-frame";
import { loadCompetitions, readCompetitions } from "@/lib/competition-list";
import { regionalCompetitionsPath } from "@/lib/competition-prefecture";
import { loadDomesticMajors, peekDomesticMajors, undecidedDomesticMeets, type DomesticMeet } from "@/lib/domestic-majors";
import { INTERNATIONAL } from "@/lib/prefectures";
import { isOngoing, isOver, seasonSchedule, todayInTokyo, type MonthGroup, type Scheduled } from "@/lib/season-schedule";
import type { Competition, DomesticMajor } from "@/lib/types";
import { RequireSession } from "./require-session";
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

export function CompetitionsHome() {
  const router = useRouter();
  const sp = useSearchParams();
  const view: View = sp.get("view") === "international" ? "international" : "domestic";

  return (
    <RequireSession>
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
        <div role="group" aria-label="表示する大会" className="grid grid-cols-2 gap-1 rounded-full border border-white/25 glass bg-white/10 p-1 md:max-w-sm">
          {views.map((item) => {
            const active = item.id === view;
            return (
              <button
                key={item.id}
                type="button"
                aria-pressed={active}
                onClick={() => router.replace(item.id === "domestic" ? "/competitions" : "/competitions?view=international")}
                className={`h-10 rounded-full text-sm font-semibold transition active:scale-95 ${
                  active ? "bg-white/25 text-white" : "text-slate-200 hover:text-white"
                }`}
              >
                {item.label}
              </button>
            );
          })}
        </div>
        {view === "domestic" ? <DomesticSchedule /> : <InternationalSchedule />}
      </div>
    </RequireSession>
  );
}

function DomesticSchedule() {
  const { token } = useAuth();
  const [majors, setMajors] = useState<DomesticMajor[] | null>(peekDomesticMajors);
  const [error, setError] = useState("");

  useEffect(() => {
    if (!token) return;
    let active = true;
    loadDomesticMajors(token)
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
  }, [token]);

  if (error) return <ErrorState message={error} />;
  if (!majors) return <CompetitionLoadingFrame />;
  return (
    <Schedule
      items={majors}
      itemKey={(major) => `${major.slug}:${major.year}`}
      renderCard={(major, today) => <DomesticCard major={major} today={today} />}
      undecided={undecidedDomesticMeets(majors)}
      emptyBody="今シーズンの国内主要大会はまだ発表されていません。"
    />
  );
}

function InternationalSchedule() {
  const { token } = useAuth();
  const cached = readCompetitions(INTERNATIONAL, 0);
  const [entry, setEntry] = useState(cached);

  useEffect(() => {
    if (!token) return;
    let active = true;
    loadCompetitions(INTERNATIONAL, 0, token).then((next) => {
      if (active) setEntry(next);
    });
    return () => {
      active = false;
    };
  }, [token]);

  if (!entry) return <CompetitionLoadingFrame />;
  if (entry.error) return <ErrorState message={entry.error} />;
  return (
    <Schedule
      items={entry.items}
      itemKey={(c) => c.id}
      renderCard={(c, today) => <InternationalCard competition={c} today={today} />}
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
  const today = todayInTokyo();
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

function CardLink({ href, over, children }: { href: string; over: boolean; children: ReactNode }) {
  if (!href) return <div className={cardBase}>{children}</div>;
  return (
    <a
      href={href}
      target="_blank"
      rel="noopener noreferrer"
      className={`${cardBase} transition hover:border-blue-700/70 hover:bg-white/16 active:scale-95 ${over ? "opacity-80" : ""}`}
    >
      {children}
    </a>
  );
}

function DomesticCard({ major, today }: { major: DomesticMajor; today: string }) {
  return (
    <CardLink href={major.site_url} over={isOver(major, today)}>
      <p className="text-xs text-slate-200">
        {major.schedule_label}
        <StateBadge item={major} today={today} />
      </p>
      <h4 className="text-lg font-semibold text-slate-100">{major.name}</h4>
      {major.venue ? <p className="mt-1 text-sm text-slate-200">{major.venue}</p> : null}
      {major.site_url ? <p className="text-sm text-blue-400">公式サイト</p> : null}
    </CardLink>
  );
}

function InternationalCard({ competition, today }: { competition: Competition; today: string }) {
  const link = destination(competition, INTERNATIONAL);
  return (
    <CardLink href={link.href} over={isOver(competition, today)}>
      <p className="text-xs text-slate-200">
        {competitionDates(competition.starts_on, competition.ends_on)}
        <StateBadge item={competition} today={today} />
      </p>
      <h4 className="text-lg font-semibold text-slate-100">{competition.name}</h4>
      {competition.location ? <p className="mt-1 text-sm text-slate-200">{competition.location}</p> : null}
      <p className="text-sm text-blue-400">{link.label}</p>
    </CardLink>
  );
}
