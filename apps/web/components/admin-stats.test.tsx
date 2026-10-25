/** @vitest-environment jsdom */
import { act } from "react";
import { createRoot } from "react-dom/client";
import { expect, it, vi } from "vitest";
import { AdminStatsView } from "./admin-stats";
import { apiFetch } from "@/lib/api";

const stats = vi.hoisted(() => ({
  recent_days: 30, weeks: [], retention: [], features: [], inquiries: [], reports: [],
  daily_started_at: "2026-10-24T16:00:00Z",
  daily: [{ day: "2026-10-25", signups: 3, posts: 4, comments: 5, likes: 6, sense_notes: 7, race_records: 8, product_reviews: 9, deletions: 2 }],
}));
vi.mock("@/lib/api", () => ({
  apiFetch: vi.fn(async (path: string) => path.endsWith("/coaching") ? { days: 90, events: [] } : stats),
}));

it("shows durable daily counts separately from live weekly statistics", async () => {
  (globalThis as typeof globalThis & { IS_REACT_ACT_ENVIRONMENT: boolean }).IS_REACT_ACT_ENVIRONMENT = true;
  const container = document.createElement("div");
  const root = createRoot(container);
  try {
    await act(async () => root.render(<AdminStatsView token="admin-token" />));
    expect(container.textContent).toContain("日ごとに残す件数");
    expect(container.textContent).toContain("初日は開始後の分だけ");
    expect(container.textContent).toContain("削除・退会後も件数は減りません");
    expect(Array.from(container.querySelectorAll("tbody tr:first-child td")).map((td) => td.textContent)).toEqual(["2026-10-25", "5", "7", "9", "2"]);
    expect(container.textContent).toContain("週ごとの動き");
  } finally {
    await act(async () => root.unmount());
  }
});

it.each(["features", "inquiries", "reports"] as const)("shows an empty state when %s is null", async (field) => {
  (globalThis as typeof globalThis & { IS_REACT_ACT_ENVIRONMENT: boolean }).IS_REACT_ACT_ENVIRONMENT = true;
  vi.mocked(apiFetch)
    .mockResolvedValueOnce({ ...stats, [field]: null })
    .mockResolvedValueOnce({ days: 90, events: [] });
  const container = document.createElement("div");
  const root = createRoot(container);
  try {
    await act(async () => root.render(<AdminStatsView token="admin-token" />));
    expect(container.textContent).toContain("週ごとの動き");
    expect(container.textContent).toContain("機能ごとの利用");
    expect(container.textContent).toContain("問い合わせの内訳");
    expect(container.textContent).toContain("通報の内訳");
    expect(container.textContent).toContain("まだありません");
  } finally {
    await act(async () => root.unmount());
  }
});

it("shows app-only usage and account counts without Umami operation totals", async () => {
  (globalThis as typeof globalThis & { IS_REACT_ACT_ENVIRONMENT: boolean }).IS_REACT_ACT_ENVIRONMENT = true;
  const features = ["post:text", "post:youtube", "post:injury", "like", "race_record", "comment", "save", "sense_note", "product_review", "practice_application", "injury_group", "menu_contest_entry", "menu_contest_vote"]
    .map((feature) => ({ feature, recent: 2, recent_users: 1, total: 3 }));
  vi.mocked(apiFetch)
    .mockResolvedValueOnce({ ...stats, features, weeks: [{ week_start: "2026-10-19", signups: 10, active_users: 4, posts: 8, sense_notes: 3, race_records: 2, deletions: 1 }] })
    .mockResolvedValueOnce({ days: 90, events: [] });
  const container = document.createElement("div");
  const root = createRoot(container);
  try {
    await act(async () => root.render(<AdminStatsView token="admin-token" />));
    const sections = Array.from(container.querySelectorAll("section"));
    const usage = sections.find((s) => s.querySelector("h2")?.textContent === "機能ごとの利用")!;
    expect(Array.from(usage.querySelectorAll("tbody tr td:first-child")).map((td) => td.textContent)).toEqual([
      "コメント", "保存", "感覚メモ", "シューズのレビュー", "練習への参加申請", "ケガのグループに参加", "エグ練の応募", "エグ練の投票",
    ]);
    const weekly = sections.find((s) => s.querySelector("h2")?.textContent === "週ごとの動き")!;
    expect(Array.from(weekly.querySelectorAll("tbody td")).map((td) => td.textContent)).toEqual(["10/19〜", "4", "3", "1"]);
    const headers = Array.from(container.querySelectorAll("th")).map((th) => th.textContent);
    for (const label of ["投稿", "いいね", "記録"]) expect(headers).not.toContain(label);
  } finally {
    await act(async () => root.unmount());
  }
});
