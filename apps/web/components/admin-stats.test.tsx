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

it("コーチングの表に★平均の列は出ない（星評価は廃止・役に立ったは残る）", async () => {
  (globalThis as typeof globalThis & { IS_REACT_ACT_ENVIRONMENT: boolean }).IS_REACT_ACT_ENVIRONMENT = true;
  const events = [{
    event_label: "100m", coaches: 2, on_duty: 1, requests: 5, returned: 3, expired: 1,
    median_hours: 10.5, rated: 4, avg_stars: 4.5, helpful_yes: 3, helpful_answers: 4,
    applied: 1, started: 1, lost: 0, started_yen: 20000,
  }];
  vi.mocked(apiFetch)
    .mockResolvedValueOnce({ ...stats })
    .mockResolvedValueOnce({ days: 90, events });
  const container = document.createElement("div");
  const root = createRoot(container);
  try {
    await act(async () => root.render(<AdminStatsView token="admin-token" />));
    const coaching = Array.from(container.querySelectorAll("section")).find(
      (s) => s.querySelector("h2")?.textContent === "コーチングの需要と供給(種目ごと)",
    )!;
    const head = Array.from(coaching.querySelectorAll("th")).map((th) => th.textContent);
    expect(head).not.toContain("★平均");
    expect(head).toContain("役に立った");
    const row = Array.from(coaching.querySelectorAll("tbody tr td")).map((td) => td.textContent);
    expect(row).toEqual(["100m", "2", "1", "5", "3", "1", "10.5時間", "75%", "1", "1", "0", "20,000円"]);
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

it("shows sense note usage without names and marks an unmeasurable repeat", async () => {
  (globalThis as typeof globalThis & { IS_REACT_ACT_ENVIRONMENT: boolean }).IS_REACT_ACT_ENVIRONMENT = true;
  const senseNotes = {
    total: 10, authors: 3, non_admin_authors: 2, profiles: 45, non_admin_profiles: 44,
    first_day: "2026-10-22", last_day: "2026-11-08",
    buckets: { one: 1, two_four: 1, five_nine: 0, ten_plus: 1 },
    active_days: { days_2_plus: 2, days_3_plus: 1, days_7_plus: 1 },
    repeat_7: { total: 2, repeat: 1 },
    repeat_28: { total: 0, repeat: 0 },
    cohorts: [{ week_start: "2026-10-19", authors: 2, next_week: 1, since_next_week: 1 }],
    weekly: [{ week_start: "2026-11-02", notes: 3, authors: 2 }],
    monthly: [{ month: "2026-11-01", notes: 10, authors: 3 }],
  };
  vi.mocked(apiFetch)
    .mockResolvedValueOnce({ ...stats, sense_notes: senseNotes })
    .mockResolvedValueOnce({ days: 90, events: [] });
  const container = document.createElement("div");
  const root = createRoot(container);
  try {
    await act(async () => root.render(<AdminStatsView token="admin-token" />));
    expect(container.textContent).toContain("感覚メモの全体");
    expect(container.textContent).toContain("2026-10-22");
    expect(container.textContent).toContain("10/19〜");
    const sections = Array.from(container.querySelectorAll("section"));
    const repeat = sections.find((s) => s.querySelector("h2")?.textContent === "感覚メモの初回からのリピート")!;
    expect(Array.from(repeat.querySelectorAll("tbody tr")).map((tr) =>
      Array.from(tr.querySelectorAll("td")).map((td) => td.textContent),
    )).toEqual([
      ["7日以内", "1", "2", "50%"],
      ["28日以内", "0", "0", "まだ測れない"],
    ]);
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
      "コメント", "保存", "感覚メモ", "シューズのレビュー", "練習への参加申請", "ケガのグループに参加", "エグ練グランプリの応募", "エグ練グランプリの投票",
    ]);
    const weekly = sections.find((s) => s.querySelector("h2")?.textContent === "週ごとの動き")!;
    expect(Array.from(weekly.querySelectorAll("tbody td")).map((td) => td.textContent)).toEqual(["10/19〜", "4", "3", "1"]);
    const headers = Array.from(container.querySelectorAll("th")).map((th) => th.textContent);
    for (const label of ["投稿", "いいね", "記録"]) expect(headers).not.toContain(label);
  } finally {
    await act(async () => root.unmount());
  }
});
