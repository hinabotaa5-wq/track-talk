"use client";

import { useEffect, useState } from "react";
import { useSearchParams } from "next/navigation";
import { CompetitionFilters } from "@/components/competition-filters";
import { MajorHostsPanel } from "@/components/major-hosts-panel";
import { EmptyState, ErrorState } from "@/components/empty-state";
import { CompetitionLoadingFrame } from "@/components/loading-frame";
import { formatDate } from "@/lib/format";
import { competitionCacheKey, loadCompetitions, readCompetitions } from "@/lib/competition-list";
import { saveCompetitionPrefecture } from "@/lib/competition-prefecture";
import { INTERNATIONAL, associationListLabel, currentMonthInTokyo, monthNumber, prefectureCode } from "@/lib/prefectures";
import type { Competition } from "@/lib/types";
import { RequireSession } from "./require-session";

function destinationLabel(competition: Competition, prefecture: string) {
  if (competition.link_label) return competition.link_label;
  if (prefecture === INTERNATIONAL) return "公式サイト";
  if (competition.organizer_url === competition.source_url) return associationListLabel(prefecture);
  return "要項";
}

function competitionDates(startsOn: string, endsOn: string | null) {
  const start = formatDate(startsOn);
  if (!endsOn) return start;
  const end = formatDate(endsOn);
  return start === end ? start : `${start} 〜 ${end}`;
}

export function CompetitionsView() {
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
    const known = readCompetitions(prefecture, month);
    if (known) {
      setItems(known.items);
      setError(known.error);
      setLoadedKey(key);
    }

    let active = true;
    loadCompetitions(prefecture, month).then((entry) => {
      if (!active) return;
      setItems(entry.items);
      setError(entry.error);
      setLoadedKey(key);
    });
    return () => {
      active = false;
    };
  }, [prefecture, month, key]);

  return (
    <RequireSession>
      <div className="space-y-4">
        <MajorHostsPanel />
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
            {shownItems.map((c) => (
              <a
                key={c.id}
                href={c.organizer_url}
                target="_blank"
                rel="noopener noreferrer"
                className="block rounded-xl border bg-white p-4 hover:border-blue-300"
              >
                <p className="text-xs text-zinc-500">{competitionDates(c.starts_on, c.ends_on)}</p>
                <h2 className="text-lg font-semibold">{c.name}</h2>
                {c.location ? <p className="mt-1 text-sm text-zinc-600">{c.location}</p> : null}
                <p className="text-sm text-blue-700">{destinationLabel(c, prefecture)}</p>
              </a>
            ))}
          </div>
        ) : null}
      </div>
    </RequireSession>
  );
}
