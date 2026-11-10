import { L } from "@/lib/i18n";
import { appEvents } from "./practice-events";
import { prefectureName, PREFECTURES } from "./prefectures";
import type { Post } from "./types";

export type PracticeList = { posts: Post[]; total: number; next_cursor: string };
export type PracticeFilter = { prefecture: string; event: string };

// 絞り込みの値として受け付けるのは、47都道府県と選べる種目だけ。それ以外は「すべて」(空)にする。
export function practiceFilterFrom(prefecture: string | null | undefined, event: string | null | undefined): PracticeFilter {
  return {
    prefecture: PREFECTURES.some((item) => item.code === prefecture) ? (prefecture as string) : "",
    event: appEvents.some((item) => item.value === event) ? (event as string) : "",
  };
}

function filterQuery(filter: PracticeFilter) {
  const params = new URLSearchParams();
  if (filter.prefecture) params.set("prefecture", filter.prefecture);
  if (filter.event) params.set("event", filter.event);
  return params;
}

// practiceListPath は API の一覧の URL。cursor があれば続きを読む。
export function practiceListPath(filter: PracticeFilter, cursor?: string) {
  const params = filterQuery(filter);
  if (cursor) params.set("cursor", cursor);
  const query = params.toString();
  return query ? `/v1/practice?${query}` : "/v1/practice";
}

// practicePagePath は画面の URL。絞り込みを URL に残し、共有や戻るで同じ一覧を開けるようにする。
export function practicePagePath(filter: PracticeFilter) {
  const query = filterQuery(filter).toString();
  return query ? `/practice?${query}` : "/practice";
}

// practiceCountLabel は「東京都の100m これからの募集 3件」のような見出し。
export function practiceCountLabel(filter: PracticeFilter, total: number) {
  const where = filter.prefecture ? prefectureName(filter.prefecture) : L("全国");
  const event = appEvents.find((item) => item.value === filter.event)?.label;
  return event
    ? L("{where}の{event} これからの募集 {total}件", { where, event, total })
    : L("{where} これからの募集 {total}件", { where, total });
}
