/** @vitest-environment jsdom */
import { act, useState } from "react";
import { createRoot, type Root } from "react-dom/client";
import { afterEach, beforeEach, expect, it, vi } from "vitest";
import { AvailabilityGrid, CoachAvailabilityEditor, PublicAvailability } from "./coach-availability";
import { availabilityDays, hasAvailability, tokyoDate, type CoachAvailability } from "@/lib/coach-availability";
const apiFetch = vi.fn();
vi.mock("@/lib/api", async (original) => ({ ...await original<typeof import("@/lib/api")>(), apiFetch: (...args: unknown[]) => apiFetch(...args) }));
const data: CoachAvailability = { today: "2026-10-30", days: [
  { date: "2026-10-30", status: "available", note: "午後のみ" }, { date: "2026-10-31", status: "consult", note: "" },
  { date: "2026-11-01", status: "unavailable", note: "" }, { date: "2026-11-02", status: "available", note: "" }, { date: "2026-11-03", status: "consult", note: "" },
] };
let root: Root; let container: HTMLDivElement;
beforeEach(() => {
  (globalThis as typeof globalThis & { IS_REACT_ACT_ENVIRONMENT: boolean }).IS_REACT_ACT_ENVIRONMENT = true;
  vi.useFakeTimers(); vi.setSystemTime(new Date("2026-10-30T00:00:00Z")); apiFetch.mockReset(); apiFetch.mockResolvedValue(data);
  container = document.createElement("div"); document.body.appendChild(container); root = createRoot(container);
});
afterEach(async () => { await act(async () => root.unmount()); container.remove(); vi.useRealTimers(); });
async function click(label: string) {
  const button = [...container.querySelectorAll("button")].find((el) => el.getAttribute("aria-label")?.startsWith(label) || el.textContent === label);
  if (!button) throw new Error(label); await act(async () => button.click());
}
it("21日を7列3段に並べ、○△×−とメモ、土曜の青・日曜の赤を表示する", async () => {
  await act(async () => root.render(<AvailabilityGrid {...data} />));
  const grid = container.querySelector('[aria-label="3週間の空き状況"]')!;
  expect(grid.classList.contains("grid-cols-7")).toBe(true); expect(grid.children.length).toBe(28);
  for (const text of ["○", "△", "×", "−", "午後のみ"]) expect(grid.textContent).toContain(text);
  expect(grid.children[1].classList.contains("text-blue-400")).toBe(true); expect(grid.children[2].classList.contains("text-red-400")).toBe(true);
  expect(grid.children[8].firstElementChild?.classList.contains("text-blue-400")).toBe(true); expect(grid.children[9].firstElementChild?.classList.contains("text-red-400")).toBe(true);
  expect(grid.lastElementChild?.textContent).toContain("11/19");
});
it("○△から3日まで選べて、選択を外すと別の日を選べる", async () => {
  function Picker() { const [dates, setDates] = useState<string[]>([]); return <PublicAvailability coachID="coach" selected={dates} onChange={setDates} />; }
  await act(async () => root.render(<Picker />));
  await click("10/30"); await click("10/31"); await click("11/2"); await click("11/3");
  expect(container.querySelectorAll('button[aria-pressed="true"]').length).toBe(3);
  expect(container.querySelector('button[aria-label^="11/1"]')?.hasAttribute("disabled")).toBe(true); expect(container.querySelector('button[aria-label^="11/4"]')?.hasAttribute("disabled")).toBe(true);
  await click("10/31"); await click("11/3");
  expect(container.querySelector('button[aria-label^="11/3"]')?.getAttribute("aria-pressed")).toBe("true");
  expect(container.textContent).toContain("3/3日選択中"); expect(container.textContent).toContain("選択だけでは確定しません");
});
it("日を選んで編集し、本人用APIに保存して未設定にも戻せる", async () => {
  await act(async () => root.render(<CoachAvailabilityEditor token="test" />));
  await click("10/31"); await click("○ 空きあり"); await click("空き状況を保存する");
  const save = apiFetch.mock.calls.find(([, options]) => options?.method === "PUT");
  expect(save?.[0]).toBe("/v1/coach/me/availability"); expect(save?.[1].token).toBe("test");
  expect(JSON.parse(save![1].body).days.find((d: { date: string }) => d.date === "2026-10-31")).toEqual({ date: "2026-10-31", status: "available", note: "" });
  expect(container.textContent).toContain("保存しました"); expect(container.querySelector('input[aria-label="空き状況のメモ"]')?.getAttribute("maxlength")).toBe("30");
  await click("− 未設定"); await click("空き状況を保存する");
  const last = apiFetch.mock.calls.at(-1)!;
  expect(JSON.parse(last[1].body).days.find((d: { date: string }) => d.date === "2026-10-31").status).toBe("");
});
it("日本時間の月末・年末で日付を進め、過去の日を含めない", () => {
  expect(tokyoDate(new Date("2026-12-31T15:00:00Z"))).toBe("2027-01-01");
  const days = availabilityDays("2026-12-31", [{ date: "2026-12-30", status: "available", note: "過去" }]);
  expect(days[0].date).toBe("2026-12-31"); expect(days[20].date).toBe("2027-01-20"); expect(days.some((d) => d.note === "過去")).toBe(false);
});
it("動画添削と従来プランにはカレンダーを付けない", () => {
  expect(hasAvailability("in_person")).toBe(true); expect(hasAvailability("competition_support")).toBe(true);
  expect(hasAvailability("video_review")).toBe(false); expect(hasAvailability(null)).toBe(false);
});
it("取得に失敗したとき、未設定と区別できる", async () => {
  apiFetch.mockRejectedValue(new Error("network")); await act(async () => root.render(<PublicAvailability coachID="coach" />));
  expect(container.querySelector('[role="alert"]')?.textContent).toContain("読み込めませんでした"); expect(container.querySelector('[aria-label="3週間の空き状況"]')).toBeNull();
});

it("日付をまたぐと過去の日の表示と選択を外す", async () => {
  vi.setSystemTime(new Date("2026-10-30T14:59:59Z"));
  function Picker() { const [dates, setDates] = useState(["2026-10-30"]); return <PublicAvailability coachID="coach" selected={dates} onChange={setDates} />; }
  await act(async () => root.render(<Picker />));
  await act(async () => vi.advanceTimersByTime(1100));
  expect(container.querySelector('button[aria-label^="10/30"]')).toBeNull();
  expect(container.textContent).toContain("0/3日選択中");
});
