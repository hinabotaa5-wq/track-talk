/** @vitest-environment jsdom */
import { act } from "react";
import { createRoot, type Root } from "react-dom/client";
import { afterEach, beforeEach, expect, it, vi } from "vitest";
import { RaceRecordsPanel } from "./race-records";
import type { RaceRecord, RaceRecords } from "@/lib/race-records";
const apiFetch = vi.hoisted(() => vi.fn());
vi.mock("@/lib/api", () => ({ apiFetch, apiErrorText: () => "保存できませんでした" }));
vi.mock("@/components/confirm-dialog", () => ({ useConfirm: () => vi.fn(), deleteConfirmOptions: vi.fn() }));
const base: RaceRecord = { id: "heat", raced_on: "2026-05-03", event: "100m", event_label: "100m", mark: 1100, round: "予選", wind: null, wind_aided: false, is_pb: true, memo: "", meet_name: "県選手権", created_at: "2026-05-03T00:00:00Z" };
const data: RaceRecords = { events: [{event:"100m",event_label:"100m",kind:"time",wind:true,count:2}, {event:"long-jump",event_label:"走幅跳",kind:"distance",wind:true,count:1}], records:[base, {...base,id:"final",round:"決勝",mark:1090}, {...base,id:"jump",event:"long-jump",event_label:"走幅跳",mark:600,round:""}] };
let root: Root, container: HTMLDivElement;
beforeEach(() => {
  (globalThis as typeof globalThis & { IS_REACT_ACT_ENVIRONMENT: boolean }).IS_REACT_ACT_ENVIRONMENT = true;
  apiFetch.mockReset(); apiFetch.mockImplementation(async (path:string) => path.includes("pb-stats") ? { events:[] } : data);
  container = document.createElement("div"); document.body.appendChild(container); root = createRoot(container);
});
afterEach(async () => { await act(async () => root.unmount()); container.remove(); });
async function render() { await act(async () => root.render(<RaceRecordsPanel token="test" />)); }
async function fill(input: HTMLInputElement, value: string) {
  await act(async () => { Object.getOwnPropertyDescriptor(HTMLInputElement.prototype,"value")!.set!.call(input,value); input.dispatchEvent(new Event("input",{bubbles:true})); });
}
it("記録会はラウンド欄を開かずに保存でき、必要なときだけ自由に追記できる", async () => {
  await render();
  const form = container.querySelector("form")!;
  const details = form.querySelector("details")!;
  const round = form.querySelector<HTMLInputElement>("input[list]")!;
  expect(details.open).toBe(false); expect(round.required).toBe(false);
  await fill(form.querySelector<HTMLInputElement>('input[placeholder^="例: 10.85"]')!, "11.12");
  await act(async () => form.dispatchEvent(new Event("submit",{bubbles:true,cancelable:true})));
  let saved = apiFetch.mock.calls.find(([,options]) => options.method === "POST");
  expect(JSON.parse(saved![1].body)).toMatchObject({round:"",mark:1112});
  await fill(round,"第2レース");
  await fill(form.querySelector<HTMLInputElement>('input[placeholder^="例: 10.85"]')!, "11.10");
  await act(async () => form.dispatchEvent(new Event("submit",{bubbles:true,cancelable:true})));
  saved = apiFetch.mock.calls.filter(([,options]) => options.method === "POST").at(-1);
  expect(JSON.parse(saved![1].body)).toMatchObject({round:"第2レース",mark:1110});
});
it("全種目を選ぶと単位を混ぜずに比較し、一覧には種目とラウンドを表示する", async () => {
  await render();
  const all = [...container.querySelectorAll("button")].find((b) => b.textContent?.startsWith("全種目"))!;
  await act(async () => all.click());
  expect(all.getAttribute("aria-pressed")).toBe("true");
  expect(container.querySelector('svg[aria-label^="全種目"]')).not.toBeNull();
  expect(container.textContent).toContain("最初の記録を100");
  expect(container.textContent).toContain("6.00m");
  expect(container.textContent).toContain("決勝");
  expect(container.querySelectorAll("ul li")).toHaveLength(3);
});
it("編集フォームは既存のラウンドを保持する", async () => {
  await render();
  await act(async () => container.querySelector<HTMLButtonElement>('[aria-label="記録を直す"]')!.click());
  const form = container.querySelectorAll("form")[1];
  const round = form.querySelector<HTMLInputElement>("input[list]")!;
  expect(round.value).toBe("決勝");
  await act(async () => form.dispatchEvent(new Event("submit",{bubbles:true,cancelable:true})));
  const saved = apiFetch.mock.calls.find(([,options]) => options.method === "PATCH");
  expect(JSON.parse(saved![1].body).round).toBe("決勝");
});
