/** @vitest-environment jsdom */
import { act } from "react";
import { createRoot, type Root } from "react-dom/client";
import { afterEach, beforeEach, expect, it, vi } from "vitest";
import { SenseNoteCalendar } from "./sense-note-calendar";
import type { RaceRecord } from "@/lib/race-records";
import type { SenseNote } from "@/lib/sense-notes";
vi.mock("@/components/confirm-dialog", () => ({ useConfirm: () => vi.fn(), deleteConfirmOptions: vi.fn() }));
vi.mock("./sense-notes", () => ({ SenseNoteItem: ({ note }: { note: SenseNote }) => <div>{note.body}</div> }));
let root: Root;
let container: HTMLDivElement;
const note: SenseNote = { id: "note", body: "リズムよく走れた", day: "2026-10-25", event: null, event_label: null, created_at: "2026-10-25T03:00:00Z", updated_at: "2026-10-25T03:00:00Z" };
const onMonthChange = vi.fn();
const onRetry = vi.fn();
beforeEach(() => {
  (globalThis as typeof globalThis & { IS_REACT_ACT_ENVIRONMENT: boolean }).IS_REACT_ACT_ENVIRONMENT = true;
  vi.useFakeTimers(); vi.setSystemTime(new Date("2026-10-25T03:00:00Z"));
  vi.clearAllMocks();
  container = document.createElement("div"); document.body.appendChild(container); root = createRoot(container);
});
afterEach(async () => { await act(async () => root.unmount()); container.remove(); vi.useRealTimers(); });
async function render(notes = [note], month = "2026-10", loading = false, failed = false) {
  await act(async () => root.render(<SenseNoteCalendar notes={notes} token="token" month={month} onMonthChange={onMonthChange} loading={loading} failed={failed} onRetry={onRetry} onDeleted={() => {}} />));
}
async function click(label: string) {
  const button = container.querySelector<HTMLButtonElement>(`button[aria-label="${label}"]`)!;
  await act(async () => button.click());
}
it("書いた日にマークが付き、タップすると詳細を開いて閉じられる", async () => {
  await render();
  expect(container.textContent).not.toContain(note.body);
  const written = container.querySelector<HTMLButtonElement>('button[aria-label="2026年10月25日、感覚メモあり"]')!;
  expect(written.disabled).toBe(false);
  expect(written.querySelector(".bg-sky-300")).not.toBeNull();
  expect(container.querySelector<HTMLButtonElement>('button[aria-label="2026年10月24日、感覚メモなし"]')!.disabled).toBe(true);
  await click("2026年10月25日、感覚メモあり");
  expect(container.querySelector('[role="dialog"]')?.textContent).toContain(note.body);
  const close = container.querySelector<HTMLButtonElement>('button[aria-label="詳細を閉じる"]')!;
  expect(document.activeElement).toBe(close);
  await act(async () => close.dispatchEvent(new KeyboardEvent("keydown", { key: "Escape", bubbles: true })));
  expect(container.querySelector('[role="dialog"]')).toBeNull();
  expect(document.activeElement).toBe(written);
  await click("2026年10月25日、感覚メモあり");
  await click("詳細を閉じる");
  expect(container.textContent).not.toContain(note.body);
});
it("月の境界を越えて移動し、未来の月への移動はできない", async () => {
  await render();
  expect(container.querySelector<HTMLButtonElement>('button[aria-label="次の月"]')!.disabled).toBe(true);
  await render([], "2026-01");
  await click("前の月");
  expect(onMonthChange).toHaveBeenLastCalledWith("2025-12");
  await render([], "2025-12");
  await click("次の月");
  expect(onMonthChange).toHaveBeenLastCalledWith("2026-01");
});
it("うるう年の月末を表示し、メモの追加・削除にマークが追従する", async () => {
  const leapNote = { ...note, day: "2024-02-29" };
  await render([], "2024-02");
  expect(container.textContent).toContain("この月の感覚メモはありません");
  expect(container.querySelector('button[aria-label="2024年2月30日、感覚メモなし"]')).toBeNull();
  await render([leapNote], "2024-02");
  await click("2024年2月29日、感覚メモあり");
  expect(container.textContent).toContain(note.body);
  await render([], "2024-02");
  expect(container.textContent).not.toContain(note.body);
  expect(container.querySelector('button[aria-label="2024年2月29日、感覚メモあり"]')).toBeNull();
});
it("未取得の月は読み込み中を表示し、失敗した場合は再読み込みできる", async () => {
  await render([], "2026-10", true);
  expect(container.textContent).toContain("この月のメモを読み込み中");
  expect(container.textContent).not.toContain("この月の感覚メモはありません");
  await render([], "2026-10", false, true);
  await act(async () => [...container.querySelectorAll("button")].find((button) => button.textContent === "再読み込み")!.click());
  expect(onRetry).toHaveBeenCalledOnce();
});

it("記録だけの日とメモ・記録の両方がある日を開ける", async () => {
  const record: RaceRecord = {id:"race", raced_on:"2026-10-24", event:"100m", event_label:"100m", mark:1100,wind:null,wind_aided:false,is_pb:true,memo:"最後まで走れた",meet_name:"記録会",round:"",created_at:"2026-10-24T00:00:00Z"};
  const props = {notes:[note],records:[record], token:"token", month:"2026-10",onMonthChange,loading:false,failed:false,onRetry,onDeleted:()=>{}};
  await act(async () => root.render(<SenseNoteCalendar {...props} />));
  await click("2026年10月24日、感覚メモなし、記録あり");
  expect(container.textContent).toContain("最後まで走れた");
  expect(container.textContent).toContain("11.00");
  await act(async () => root.render(<SenseNoteCalendar {...props} records={[{...record,raced_on:note.day}]} />));
  await click("2026年10月25日、感覚メモあり、記録あり");
  expect(container.textContent).toContain(note.body);
  expect(container.textContent).toContain("最後まで走れた");
  await act(async () => root.render(<SenseNoteCalendar {...props} records={[]} />));
  expect(container.textContent).not.toContain("最後まで走れた");
  expect(container.textContent).toContain(note.body);
});

it("元のメモを削除した後も、まとめだけの日をカレンダーから開ける", async () => {
  const summary = {
    id: "summary", from_day: "2026-10-01", to_day: "2026-10-06", note_count: 6,
    summary: { overview: "接地が安定した", good: ["リズムがよい"], concerns: [], next: ["腕振り"] },
    notes: [note], created_at: "2026-10-07T00:00:00Z",
  };
  await act(async () => root.render(<SenseNoteCalendar notes={[]} summaries={[summary]} token="token" month="2026-10" onMonthChange={onMonthChange} loading={false} failed={false} onRetry={onRetry} onDeleted={() => {}} />));
  const day = container.querySelector<HTMLButtonElement>('button[aria-label="2026年10月6日、感覚メモなし、まとめた感覚あり"]')!;
  expect(day.disabled).toBe(false);
  expect(day.querySelector(".bg-violet-300")).not.toBeNull();
  expect(container.textContent).not.toContain("この月の感覚メモはありません");
  await act(async () => day.click());
  const dialog = container.querySelector('[role="dialog"]')!;
  expect(dialog.textContent).toContain("接地が安定した");
  expect(dialog.textContent).toContain(note.body);
  expect(dialog.querySelector("details")!.open).toBe(true);
  await act(async () => dialog.parentElement!.click());
  expect(container.querySelector('[role="dialog"]')).toBeNull();
});
