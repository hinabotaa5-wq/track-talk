import { L } from "@/lib/i18n";
const videoReviewStart = Date.parse("2026-11-14T00:00:00+09:00");

export function videoReviewStartNotice() {
  return Date.now() < videoReviewStart
    ? L("動画添削は 2026年11月14日（土）に開始予定です。")
    : null;
}
