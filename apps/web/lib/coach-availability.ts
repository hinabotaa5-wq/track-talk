import { L, weekdayLabel } from "@/lib/i18n";

// 空き日カレンダーは廃止した（2026-11-13）。契約に保存済みの希望日（preferred_dates）を表示するための日付表記だけ残す。
export function availabilityDateLabel(date: string): string {
  const day = new Date(`${date}T00:00:00Z`);
  return L("{v0}/{v1}（{v2}）", { v0: day.getUTCMonth() + 1, v1: day.getUTCDate(), v2: weekdayLabel(day.getUTCDay()) });
}
