/** @vitest-environment jsdom */
import { act } from "react";
import { createRoot, type Root } from "react-dom/client";
import { afterEach, beforeEach, expect, it, vi } from "vitest";
import { RaceRecordRow, RaceRecordsPanel } from "./race-records";
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
it("ラウンド欄は大会名の下に常に出し、空欄でも保存でき、必要なときだけ自由に追記できる", async () => {
  await render();
  const form = container.querySelector("form")!;
  expect(form.querySelector("details")).toBeNull();
  expect(form.textContent).not.toContain("記録会などは空欄のまま");
  const round = form.querySelector<HTMLInputElement>("input[list]")!;
  const meet = form.querySelector<HTMLInputElement>('input[placeholder="例: 県高校総体"]')!;
  expect(meet.compareDocumentPosition(round) & Node.DOCUMENT_POSITION_FOLLOWING).toBeTruthy();
  expect(round.required).toBe(false);
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
it("全種目を選ぶと各種目のグラフを縦に並べ、記録カードの一覧は出さない", async () => {
  await render();
  expect(container.querySelector('[aria-label="記録を直す"]')).toBeNull();
  expect(container.textContent).not.toContain("小さな変化を大きく見せない");
  const all = [...container.querySelectorAll("button")].find((b) => b.textContent?.startsWith("全種目"))!;
  await act(async () => all.click());
  expect(all.getAttribute("aria-pressed")).toBe("true");
  expect([...container.querySelectorAll("svg[aria-label]")].map((svg) => svg.getAttribute("aria-label"))).toEqual(["100m記録の推移のグラフ", "走幅跳記録の推移のグラフ"]);
  expect(container.textContent).toContain("6.00");
  expect(container.querySelector("ul")).toBeNull();
});
it("グラフに日付は出さず、点を押すとその記録の日付を表示する", async () => {
  await render();
  const svg = container.querySelector("svg[aria-label]")!;
  expect(svg.textContent).not.toContain("5月3日");
  const point = svg.querySelector<SVGGElement>('[role="button"]')!;
  await act(async () => point.dispatchEvent(new MouseEvent("click", { bubbles: true })));
  expect(svg.textContent).toContain("5月3日");
  await act(async () => point.dispatchEvent(new MouseEvent("click", { bubbles: true })));
  expect(svg.textContent).not.toContain("5月3日");
});
it("未来の日付は「〇〇以下に設定してください」と案内する", async () => {
  await render();
  const date = container.querySelector<HTMLInputElement>('input[type="date"]')!;
  Object.defineProperty(date, "validity", { configurable: true, get: () => ({ rangeOverflow: true }) });
  await act(async () => date.dispatchEvent(new Event("invalid", { cancelable: true })));
  expect(date.validationMessage).toMatch(/^\d{4}\/\d{2}\/\d{2}以下に設定してください$/);
});
it("カレンダーの詳細から開く編集フォームは既存のラウンドを保持する", async () => {
  await act(async () => root.render(<RaceRecordRow record={data.records[1]} kind="time" token="test" onChanged={() => {}} />));
  await act(async () => container.querySelector<HTMLButtonElement>('[aria-label="記録を直す"]')!.click());
  const form = container.querySelector("form")!;
  const round = form.querySelector<HTMLInputElement>("input[list]")!;
  expect(round.value).toBe("決勝");
  await act(async () => form.dispatchEvent(new Event("submit",{bubbles:true,cancelable:true})));
  const saved = apiFetch.mock.calls.find(([,options]) => options.method === "PATCH");
  expect(JSON.parse(saved![1].body).round).toBe("決勝");
});
