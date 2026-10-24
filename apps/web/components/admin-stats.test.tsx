/** @vitest-environment jsdom */
import { act } from "react";
import { createRoot } from "react-dom/client";
import { expect, it, vi } from "vitest";
import { AdminStatsView } from "./admin-stats";

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
    expect(Array.from(container.querySelectorAll("tbody tr:first-child td")).map((td) => td.textContent)).toEqual(["2026-10-25", "3", "4", "5", "6", "7", "8", "9", "2"]);
    expect(container.textContent).toContain("週ごとの動き");
  } finally {
    await act(async () => root.unmount());
  }
});
