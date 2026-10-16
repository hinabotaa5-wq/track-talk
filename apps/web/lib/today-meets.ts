// 今日開催中の大会のうち、ホームのヘッダーの吹き出しで知らせるもの。
// 国内主要大会はすべて、国際大会はオリンピック・世界陸上・U20世界陸上・ダイヤモンドリーグ・アジア大会だけを出す。

import { isOngoing } from "./season-schedule";
import type { Competition, DomesticMajor } from "./types";

export type TodayMeet = { key: string; name: string; href: string };

// 国際大会の名前は取り込み時に日本語へ置き換わっているが、置き換えられなかった英語名も拾う。
const featuredNames = [
  /^オリンピック/,
  /^世界陸上/,
  /^U20世界陸上/,
  /ダイヤモンドリーグ/,
  /アジア(競技)?大会/,
  /^(Summer )?Olympic Games/i,
  /^World Athletics (U20 )?Championships/i,
  /Diamond League/i,
  /Asian Games/i,
];

function isDiamondLeagueSite(raw: string) {
  try {
    const host = new URL(raw).hostname;
    return host === "diamondleague.com" || host.endsWith(".diamondleague.com");
  } catch {
    return false;
  }
}

// ダイヤモンドリーグの各大会は「ゴールデンガラ」のように大会名だけになることがあるので、公式サイトでも見分ける。
export function isFeaturedInternational(competition: Competition) {
  const name = competition.name.trim();
  if (featuredNames.some((pattern) => pattern.test(name))) return true;
  return isDiamondLeagueSite(competition.organizer_url) || isDiamondLeagueSite(competition.source_url);
}

export function todayMeets(domestic: DomesticMajor[], international: Competition[], today: string): TodayMeet[] {
  const meets: TodayMeet[] = [];
  for (const major of domestic) {
    if (!isOngoing(major, today)) continue;
    meets.push({ key: `${major.slug}:${major.year}`, name: major.name, href: major.site_url || "/competitions" });
  }
  for (const competition of international) {
    if (!isOngoing(competition, today) || !isFeaturedInternational(competition)) continue;
    meets.push({
      key: competition.id,
      name: competition.name,
      href: competition.guidelines_url || competition.organizer_url || "/competitions?view=international",
    });
  }
  return meets;
}

export function balloonMessage(meets: TodayMeet[]) {
  return `今日は${meets.map((meet) => meet.name).join("、")}が開催されてるよ！`;
}
