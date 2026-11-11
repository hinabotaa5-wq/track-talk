import { L } from "@/lib/i18n";

// 県別の大会一覧は #549 で廃止したため、大会カードの日付とリンクの見せ方は
// 国際大会の一覧でだけ使う。
import { formatDate } from "@/lib/format";
import type { Competition } from "@/lib/types";

export function competitionDestination(competition: Competition) {
  // 新着から補った最新の要項を優先し、無ければ年間表の要項を出す。
  if (competition.guidelines_url) return { href: competition.guidelines_url, label: L("要項") };
  if (competition.link_label === "要項") return { href: competition.organizer_url, label: L("要項") };
  if (competition.link_label) return { href: competition.organizer_url, label: competition.link_label };
  return { href: competition.organizer_url, label: L("公式サイト") };
}

export function competitionDates(startsOn: string, endsOn: string | null) {
  const start = formatDate(startsOn);
  if (!endsOn) return start;
  const end = formatDate(endsOn);
  return start === end ? start : L("{start} 〜 {end}", { start, end });
}
