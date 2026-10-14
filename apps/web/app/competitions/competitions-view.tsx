"use client";

import { useEffect, useState } from "react";
import Link from "next/link";
import { useSearchParams } from "next/navigation";
import { useAuth } from "@/components/auth-provider";
import { CompetitionFilters } from "@/components/competition-filters";
import { EmptyState, ErrorState } from "@/components/empty-state";
import { CompetitionLoadingFrame } from "@/components/loading-frame";
import { formatDate } from "@/lib/format";
import { competitionCacheKey, loadCompetitions, readCompetitions } from "@/lib/competition-list";
import { saveCompetitionPrefecture } from "@/lib/competition-prefecture";
import { INTERNATIONAL, associationListLabel, currentMonthInTokyo, monthNumber, prefectureCode } from "@/lib/prefectures";
import type { Competition } from "@/lib/types";
import { RequireSession } from "./require-session";

function destination(competition: Competition, prefecture: string) {
  // 年間表の要項を優先し、無ければ新着から補った要項を出す。
  if (competition.link_label === "要項") return { href: competition.organizer_url, label: "要項" };
  if (competition.guidelines_url) return { href: competition.guidelines_url, label: "要項" };
  if (competition.link_label) return { href: competition.organizer_url, label: competition.link_label };
  if (prefecture === INTERNATIONAL) return { href: competition.organizer_url, label: "公式サイト" };
  if (competition.organizer_url === competition.source_url) {
    return { href: competition.organizer_url, label: associationListLabel(prefecture) };
  }
  return { href: competition.organizer_url, label: "要項" };
}

function competitionDates(startsOn: string, endsOn: string | null) {
  const start = formatDate(startsOn);
  if (!endsOn) return start;
  const end = formatDate(endsOn);
  return start === end ? start : `${start} 〜 ${end}`;
}

export function CompetitionsView() {
  const { token } = useAuth();
  const sp = useSearchParams();
  const month = monthNumber(sp.get("month") ?? undefined) || currentMonthInTokyo();
  const fromUrl = prefectureCode(sp.get("prefecture") ?? undefined);
  const prefecture = fromUrl || INTERNATIONAL;
  const key = competitionCacheKey(prefecture, month);
  const cached = readCompetitions(prefecture, month);
  const [loadedKey, setLoadedKey] = useState(cached ? key : "");
  const [items, setItems] = useState<Competition[]>(cached?.items ?? []);
  const [error, setError] = useState(cached?.error ?? "");
  const ready = loadedKey === key;
  const shownItems = ready ? items : (cached?.items ?? []);
  const shownError = ready ? error : (cached?.error ?? "");
  const loading = !ready && !cached;

  useEffect(() => {
    if (fromUrl) saveCompetitionPrefecture(fromUrl);
  }, [fromUrl]);

  useEffect(() => {
    if (!token) return;
    let active = true;
    loadCompetitions(prefecture, month, token).then((entry) => {
      if (!active) return;
      setItems(entry.items);
      setError(entry.error);
      setLoadedKey(key);
    });
    return () => {
      active = false;
    };
  }, [prefecture, month, key, token]);

  return (
    <RequireSession>
      <div className="space-y-4">
        <Link
          href="/competitions/major-hosts"
          className="inline-flex h-11 w-full items-center justify-center rounded-full border border-white/25 glass bg-white/12 px-4.5 text-sm font-semibold text-slate-200 shadow-xs transition hover:bg-white/16 hover:text-white active:scale-95 md:w-fit"
        >
          今後の世界陸上・オリンピック
        </Link>
        <CompetitionFilters month={month} prefecture={prefecture} />
        {loading ? <CompetitionLoadingFrame /> : null}
        {!loading && shownError ? <ErrorState message={shownError} /> : null}
        {!loading && !shownError && shownItems.length === 0 ? (
          <EmptyState
            title="この月の大会はありません"
            body={prefecture === INTERNATIONAL ? "別の月を選ぶと、他の国際大会を表示します。" : "別の月か都道府県を選んでください。"}
          />
        ) : null}
        {!loading && shownItems.length > 0 ? (
          <div className="grid gap-3 md:grid-cols-2">
            {shownItems.map((c) => {
              const link = destination(c, prefecture);
              return (
                <a
                  key={c.id}
                  href={link.href}
                  target="_blank"
                  rel="noopener noreferrer"
                  className="block rounded-xl border border-white/20 glass bg-white/10 p-4 transition hover:border-blue-700/70 hover:bg-white/16 active:scale-95"
                >
                  <p className="text-xs text-slate-200">{competitionDates(c.starts_on, c.ends_on)}</p>
                  <h2 className="text-lg font-semibold text-slate-100">{c.name}</h2>
                  {c.location ? <p className="mt-1 text-sm text-slate-200">{c.location}</p> : null}
                  <p className="text-sm text-blue-400">{link.label}</p>
                </a>
              );
            })}
          </div>
        ) : null}
      </div>
    </RequireSession>
  );
}
