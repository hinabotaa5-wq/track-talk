import { apiFetch } from "@/lib/api";
import { formatDate } from "@/lib/format";
import { INTERNATIONAL, associationListLabel, currentMonthInTokyo, monthNumber, prefectureCode } from "@/lib/prefectures";
import { CompetitionFilters } from "@/components/competition-filters";
import { EmptyState, ErrorState } from "@/components/empty-state";
import type { Competition } from "@/lib/types";

export const dynamic = "force-dynamic";

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

export default async function CompetitionsPage({
  searchParams,
}: {
  searchParams: Promise<{ prefecture?: string; month?: string }>;
}) {
  const sp = await searchParams;
  const month = monthNumber(sp.month) || currentMonthInTokyo();
  const prefecture = prefectureCode(sp.prefecture) || INTERNATIONAL;

  let items: Competition[] = [];
  let error = "";
  try {
    const data = await apiFetch<{ competitions: Competition[]; season_year: number; month: number }>(
      `/v1/competitions?prefecture=${prefecture}&month=${month}`,
    );
    items = data.competitions ?? [];
  } catch {
    error = "大会一覧を取得できませんでした。";
  }

  return (
    <div className="space-y-4">
      <h1 className="text-2xl font-bold">大会</h1>
      <p className="text-sm text-zinc-600">
        月と地域を選ぶと、その月の大会を日付の早い順に表示します。最初は国際大会です。会場も表示します。国際大会は公式サイトか大会ページへ、国内大会は要項があれば要項へ、無ければ地区のページへ、どちらも無ければその県の陸協ホームページへ移動します。
      </p>
      <CompetitionFilters month={month} prefecture={prefecture} />
      {error ? <ErrorState message={error} /> : null}
      {!error && items.length === 0 ? (
        <EmptyState
          title="この月の大会はありません"
          body={prefecture === INTERNATIONAL ? "別の月を選ぶと、他の国際大会を表示します。" : "別の月か都道府県を選んでください。"}
        />
      ) : null}
      {items.length > 0 ? (
        <div className="space-y-3">
          {items.map((c) => (
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
  );
}
