/** @vitest-environment jsdom */
import { act } from "react";
import { createRoot, type Root } from "react-dom/client";
import { afterEach, beforeEach, expect, it, vi } from "vitest";
import MySenseNotesPage from "@/app/memo/page";
import type { SenseNote } from "@/lib/sense-notes";
const apiFetch = vi.fn();
vi.mock("next/navigation", () => ({ useRouter: () => ({ push: vi.fn() }) }));
vi.mock("./auth-provider", () => ({ useAuth: () => ({ session: {}, token: "token", loading: false }) }));
vi.mock("@/lib/api", () => ({ apiFetch: (...args: unknown[]) => args[0] === "/v1/me/race-records" ? Promise.resolve({ records: [], events: [] }) : apiFetch(...args) }));
vi.mock("@/lib/features", () => ({ senseSummaryEnabled: false }));
vi.mock("./race-records", () => ({ RaceRecordsPanel: () => null }));
vi.mock("./sense-note-jar", () => ({ SenseNoteJar: () => null }));
vi.mock("./view-switch", () => ({ ViewSwitch: () => null }));
vi.mock("./sense-notes", () => ({ SenseNoteComposer: () => null, SenseNoteStatsRow: () => null, SenseNoteItem: ({ note }: { note: SenseNote }) => <p>{note.body}</p> }));
let root: Root;
let container: HTMLDivElement;
function note(day: string): SenseNote { return { id: day, day, body: `メモ${day}`, event: null, event_label: null, created_at: `${day}T03:00:00Z`, updated_at: `${day}T03:00:00Z` }; }
function page(notes: SenseNote[], cursor: string | null) { return { notes, next_cursor: cursor, stats: { total: 80, days: 80, streak: 1, today: 1 } }; }
beforeEach(() => {
  (globalThis as typeof globalThis & { IS_REACT_ACT_ENVIRONMENT: boolean }).IS_REACT_ACT_ENVIRONMENT = true;
  vi.useFakeTimers(); vi.setSystemTime(new Date("2026-10-31T03:00:00Z")); apiFetch.mockReset();
  container = document.createElement("div"); document.body.appendChild(container); root = createRoot(container);
});
afterEach(async () => { await act(async () => root.unmount()); container.remove(); vi.useRealTimers(); });
async function click(label: string) { await act(async () => container.querySelector<HTMLButtonElement>(`button[aria-label="${label}"]`)!.click()); }
it("31日ある月の最初の30件だけで止まらず、前月へ移動するとさらに読み込む", async () => {
  apiFetch.mockResolvedValueOnce(page(Array.from({ length: 30 }, (_, i) => note(`2026-10-${String(31 - i).padStart(2, "0")}`)), "october"))
    .mockResolvedValueOnce(page([note("2026-10-01"), note("2026-09-30")], "september"))
    .mockResolvedValueOnce(page([note("2026-09-01"), note("2026-08-31")], "august"));
  await act(async () => root.render(<MySenseNotesPage />));
  expect(apiFetch).toHaveBeenCalledTimes(2);
  await click("2026年10月1日、感覚メモあり、記録なし");
  expect(container.textContent).toContain("メモ2026-10-01");
  await click("前の月");
  expect(apiFetch).toHaveBeenCalledTimes(3);
  await click("2026年9月1日、感覚メモあり、記録なし");
  expect(container.textContent).toContain("メモ2026-09-01");
});
it("過去メモの取得失敗を表示し、再読み込みで不足分を取得する", async () => {
  apiFetch.mockResolvedValueOnce(page([note("2026-10-31")], "older"))
    .mockRejectedValueOnce(new Error("offline"))
    .mockResolvedValueOnce(page([note("2026-10-01")], null));
  await act(async () => root.render(<MySenseNotesPage />));
  expect(container.textContent).toContain("過去のメモを読み込めませんでした");
  await act(async () => [...container.querySelectorAll("button")].find((button) => button.textContent === "再読み込み")!.click());
  await click("2026年10月1日、感覚メモあり、記録なし");
  expect(container.textContent).toContain("メモ2026-10-01");
  expect(container.textContent).not.toContain("過去のメモを読み込めませんでした");
});
