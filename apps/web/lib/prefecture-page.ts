// 県ページ: その県のこれからの大会と、クラブチームの募集を1ページに集める。

import { homeHref } from "./home-sort";
import { PREFECTURES } from "./prefectures";

export type PrefecturePage = {
  prefecture: { code: string; name: string };
  competitions: { id: string; name: string; starts_on: string; ends_on: string | null; location: string }[];
  competition_total: number;
  recruitments: { id: string; title: string; created_at: string; venue: string; schedule: string }[];
  recruitment_total: number;
};

// 県ページがある都道府県か("00" の国際大会は県ページを持たない)。
export function isPrefecturePageCode(code: string) {
  return code !== "00" && PREFECTURES.some((item) => item.code === code);
}

export function prefecturePageHref(code: string) {
  return `/prefectures/${code}`;
}

// 大会の「もっと見る」: 大会タブの県の一覧を、最初の大会の月で開く。
export function prefectureCompetitionsHref(code: string, firstStartsOn?: string) {
  const month = firstStartsOn ? Number(firstStartsOn.slice(5, 7)) : 0;
  return `/competitions/regional?prefecture=${code}${month ? `&month=${month}` : ""}`;
}

// 募集の「もっと見る」: ホームの募集を県名で検索する(検索は県名で募集の都道府県も拾う)。
export function prefectureRecruitmentsHref(name: string) {
  return homeHref("recruitment", name);
}
