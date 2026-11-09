/** @vitest-environment jsdom */
import { act } from "react";
import { createRoot, type Root } from "react-dom/client";
import { afterEach, beforeEach, expect, it } from "vitest";
import { coachEventLabels, coachEvents } from "@/lib/coach-recruit";
import type { CoachEvent } from "@/lib/types";
import { CoachEventChips, coachEventFilterOrder, filterCoachesByEvent, type CoachEventFilter } from "./coach-event-filter";

let root: Root;
let container: HTMLDivElement;
beforeEach(() => {
  (globalThis as typeof globalThis & { IS_REACT_ACT_ENVIRONMENT: boolean }).IS_REACT_ACT_ENVIRONMENT = true;
  container = document.createElement("div"); document.body.appendChild(container); root = createRoot(container);
});
afterEach(async () => { await act(async () => root.unmount()); container.remove(); });

it("並べる順は担当種目のマスタと同じ種目をちょうど1回ずつ含む（二重のマスタにしない）", () => {
  expect([...coachEventFilterOrder].sort()).toEqual([...coachEvents].sort());
  expect(new Set(coachEventFilterOrder).size).toBe(coachEventFilterOrder.length);
});

it("近い種目が隣り合う区分順に並ぶ", () => {
  expect(coachEventFilterOrder.map((e) => coachEventLabels[e])).toEqual([
    "100m", "200m", "400m", "800m", "1500m", "5000m", "10000m", "3000m障害物", "100mハードル", "110mハードル", "400mハードル",
    "走高跳", "棒高跳", "走幅跳", "三段跳", "砲丸投", "円盤投", "ハンマー投", "やり投", "十種競技", "七種競技", "競歩", "リレー",
  ]);
});

const coaches: { id: string; events: CoachEvent[] }[] = [
  { id: "c", events: ["long-jump"] },
  { id: "a", events: ["100m", "200m", "400m"] },
  { id: "b", events: ["200m"] },
];

it("「すべて」は全員を受け取った順のまま返す", () => {
  expect(filterCoachesByEvent(coaches, "all").map((c) => c.id)).toEqual(["c", "a", "b"]);
});

it("種目を選ぶと、その種目を宣言したコーチだけを受け取った順のまま返す。複数種目のコーチは各種目で出る", () => {
  expect(filterCoachesByEvent(coaches, "100m").map((c) => c.id)).toEqual(["a"]);
  expect(filterCoachesByEvent(coaches, "200m").map((c) => c.id)).toEqual(["a", "b"]);
  expect(filterCoachesByEvent(coaches, "400m").map((c) => c.id)).toEqual(["a"]);
  expect(filterCoachesByEvent(coaches, "javelin")).toEqual([]);
});

it("チップは1列で折り返さず横にスクロールし、先頭の「すべて」から全種目が並ぶ。選択中は色以外（✓）でも分かる", async () => {
  let value: CoachEventFilter = "all";
  const render = () => root.render(<CoachEventChips value={value} onChange={(v) => { value = v; render(); }} />);
  await act(async () => render());
  const group = container.querySelector('[role="radiogroup"]')!;
  expect(group.className).toContain("flex-nowrap");
  expect(group.className).toContain("overflow-x-auto");
  expect(group.className).not.toContain("sticky");
  const radios = [...group.querySelectorAll('[role="radio"]')];
  expect(radios.map((r) => r.textContent?.replace("✓", ""))).toEqual(["すべて", ...coachEventFilterOrder.map((e) => coachEventLabels[e])]);
  expect(radios.every((r) => r.className.includes("shrink-0") && r.className.includes("whitespace-nowrap"))).toBe(true);
  expect(group.querySelector('[aria-checked="true"]')?.textContent).toBe("✓すべて");
  await act(async () => { (radios.find((r) => r.textContent === "走幅跳") as HTMLButtonElement).click(); });
  expect(value).toBe("long-jump");
  const checked = [...container.querySelectorAll('[aria-checked="true"]')];
  expect(checked.map((r) => r.textContent)).toEqual(["✓走幅跳"]);
});
