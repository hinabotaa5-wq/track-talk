/** @vitest-environment jsdom */
import { act } from "react";
import { createRoot, type Root } from "react-dom/client";
import { afterEach, beforeEach, expect, it, vi } from "vitest";
import type { SenseSummary } from "@/lib/sense-notes";
import { SenseSummaryCard } from "./sense-summary";

const apiFetch = vi.fn();
vi.mock("@/lib/api", async (original) => ({ ...await original<typeof import("@/lib/api")>(), apiFetch: (...args: unknown[]) => apiFetch(...args) }));
const saved: SenseSummary = {
  id: "summary-1", from_day: "2026-10-01", to_day: "2026-10-06", note_count: 6,
  summary: { overview: "リズムをつかめてきた", good: ["接地が安定した"], concerns: [], next: ["腕振りを意識する"] },
  notes: [{ id: "note-1", day: "2026-10-01", body: "真下に押せた\n30m×5", event: "100m", event_label: "100m", created_at: "2026-10-01T00:00:00Z", updated_at: "2026-10-01T00:00:00Z" }],
  created_at: "2026-10-06T00:00:00Z",
};
let root: Root;
let container: HTMLDivElement;
beforeEach(() => {
  (globalThis as typeof globalThis & { IS_REACT_ACT_ENVIRONMENT: boolean }).IS_REACT_ACT_ENVIRONMENT = true;
  container = document.createElement("div"); document.body.appendChild(container); root = createRoot(container);
  apiFetch.mockReset();
});
afterEach(async () => { await act(async () => root.unmount()); container.remove(); });
async function render() {
  await act(async () => root.render(<SenseSummaryCard token="test" total={12} />));
}
async function click(label: string) {
  const button = [...container.querySelectorAll("button")].find((el) => el.textContent === label);
  if (!button) throw new Error(`ボタンがありません: ${label}`);
  await act(async () => button.click());
}
it("履歴を開くと要約と元の感覚を読める", async () => {
  apiFetch.mockImplementation(async (path: string) => path.endsWith("/summaries")
    ? { summaries: [saved], next_cursor: null } : { pending_notes: 0, min_notes: 6 });
  await render();
  const details = container.querySelector("details")!;
  expect(details.open).toBe(false);
  await act(async () => details.querySelector("summary")!.click());
  expect(details.open).toBe(true);
  expect(details.textContent).toContain("接地が安定した");
  expect(details.textContent).toContain("真下に押せた\n30m×5");
  expect(details.textContent).toContain("100m");
});
it("作成した要約を先頭に追加し、過去の履歴も読み込める", async () => {
  const created = { ...saved, id: "summary-2", summary: { ...saved.summary, overview: "新しいまとめ" } };
  apiFetch.mockImplementation(async (path: string, options?: { method?: string }) => {
    if (options?.method === "POST") return { summary: created, pending_notes: 0 };
    if (path.includes("cursor=")) return { summaries: [{ ...saved, id: "summary-0" }], next_cursor: null };
    if (path.endsWith("/summaries")) return { summaries: [saved], next_cursor: "older" };
    return { pending_notes: 6, min_notes: 6 };
  });
  await render();
  await click("6個の感覚をまとめる");
  expect(container.querySelector("details")?.textContent).toContain("新しいまとめ");
  expect(container.querySelectorAll("details")).toHaveLength(2);
  expect(container.textContent).toContain("あと6個");
  await click("過去のまとめをもっと見る");
  expect(container.querySelectorAll("details")).toHaveLength(3);
  expect(container.textContent).not.toContain("過去のまとめをもっと見る");
});
it("古い要約は元の感覚が未保存と表示し、取得に失敗したら再試行できる", async () => {
  apiFetch.mockRejectedValue(new Error("offline"));
  await render();
  expect(container.querySelector('[role="alert"]')?.textContent).toContain("読み込めませんでした");
  apiFetch.mockImplementation(async (path: string) => path.endsWith("/summaries")
    ? { summaries: [{ ...saved, notes: null }], next_cursor: null } : { pending_notes: 0, min_notes: 6 });
  await click("もう一度読み込む");
  expect(container.textContent).toContain("保存する前に作成されたものです");
});
it("作成前に始まった再読み込みが遅れても、新しいまとめを消さない", async () => {
  apiFetch.mockImplementation(async (path: string) => path.endsWith("/summaries")
    ? { summaries: [saved], next_cursor: null } : { pending_notes: 6, min_notes: 6 });
  await render();
  let resolveHistory!: (value: unknown) => void;
  const oldHistory = new Promise((resolve) => { resolveHistory = resolve; });
  const created = { ...saved, id: "new", summary: { ...saved.summary, overview: "新しいまとめ" } };
  apiFetch.mockImplementation(async (path: string, options?: { method?: string }) => {
    if (options?.method === "POST") return { summary: created, pending_notes: 1 };
    if (path.endsWith("/summaries")) return oldHistory;
    return { pending_notes: 7, min_notes: 6 };
  });
  await act(async () => root.render(<SenseSummaryCard token="test" total={13} />));
  await click("6個の感覚をまとめる");
  await act(async () => resolveHistory({ summaries: [saved], next_cursor: null }));
  expect(container.querySelectorAll("details")).toHaveLength(2);
  expect(container.querySelector("details")?.textContent).toContain("新しいまとめ");
  expect(container.textContent).toContain("あと5個");
});
