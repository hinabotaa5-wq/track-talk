// 県ページ: その県のクラブチームの募集を集める。県別の大会の一覧は #549 で廃止した。

import { homeHref } from "./home-sort";
import { PREFECTURES } from "./prefectures";

export type PrefecturePage = {
  prefecture: { code: string; name: string };
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

// 募集の「もっと見る」: ホームの募集を県名で検索する(検索は県名で募集の都道府県も拾う)。
export function prefectureRecruitmentsHref(name: string) {
  return homeHref("recruitment", name);
}
