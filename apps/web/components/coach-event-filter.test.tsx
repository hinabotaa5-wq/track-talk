/** @vitest-environment jsdom */
import { act } from "react";
import { createRoot, type Root } from "react-dom/client";
import { afterEach, beforeEach, expect, it, vi } from "vitest";
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

it("チップは1列で折り返さず横にスクロールし、先頭の「すべて」から全種目が並ぶ。選択中はaria-checkedと色で分かり、表示はラベルのみ", async () => {
  let value: CoachEventFilter = "all";
  const render = () => root.render(<CoachEventChips value={value} onChange={(v) => { value = v; render(); }} />);
  await act(async () => render());
  const group = container.querySelector('[role="radiogroup"]')!;
  expect(group.className).toContain("flex-nowrap");
  expect(group.className).toContain("overflow-x-auto");
  expect(group.className).not.toContain("sticky");
  const radios = [...group.querySelectorAll('[role="radio"]')];
  expect(radios.map((r) => r.textContent)).toEqual(["すべて", ...coachEventFilterOrder.map((e) => coachEventLabels[e])]);
  expect(radios.every((r) => r.className.includes("shrink-0") && r.className.includes("whitespace-nowrap"))).toBe(true);
  expect(radios.every((r) => !r.textContent?.includes("✓"))).toBe(true);
  const initialChecked = group.querySelector('[aria-checked="true"]')!;
  expect(initialChecked.textContent).toBe("すべて");
  expect(initialChecked.className).toContain("bg-blue-500/75");
  expect(initialChecked.className).not.toContain("bg-blue-600");
  expect([...group.querySelectorAll('[aria-checked="false"]')].every((r) => r.className.includes("glass"))).toBe(true);
  await act(async () => { (radios.find((r) => r.textContent === "走幅跳") as HTMLButtonElement).click(); });
  expect(value).toBe("long-jump");
  const checked = [...container.querySelectorAll('[aria-checked="true"]')];
  expect(checked.map((r) => r.textContent)).toEqual(["走幅跳"]);
  expect(checked.every((r) => !r.textContent?.includes("✓"))).toBe(true);
  expect(checked[0].className).toContain("bg-blue-500/75");
  expect(checked[0].className).not.toContain("bg-blue-600");
});

// #676 PC向け操作のテスト。jsdom にレイアウトはないので、scroll の寸法は直接持たせる。
function mockScrollMetrics(el: HTMLElement, { scrollLeft = 0, clientWidth = 300, scrollWidth = 1200 } = {}) {
  let left = scrollLeft;
  Object.defineProperty(el, "scrollLeft", { get: () => left, set: (v: number) => { left = v; }, configurable: true });
  Object.defineProperty(el, "clientWidth", { value: clientWidth, configurable: true });
  Object.defineProperty(el, "scrollWidth", { value: scrollWidth, configurable: true });
  const scrollBy = vi.fn((options: ScrollToOptions) => {
    if (options.left !== undefined) left = Math.max(0, left + options.left);
  });
  const scrollTo = vi.fn((options: ScrollToOptions) => {
    if (options.left !== undefined) left = options.left;
  });
  (el as unknown as Record<string, unknown>).scrollBy = scrollBy;
  (el as unknown as Record<string, unknown>).scrollTo = scrollTo;
  return { scrollBy, scrollTo, getLeft: () => left };
}

function renderChips(onChange: (v: CoachEventFilter) => void = () => {}, value: CoachEventFilter = "all") {
  root.render(<CoachEventChips value={value} onChange={onChange} />);
}

function byLabel(label: string) {
  return container.querySelector(`[aria-label="${label}"]`) as HTMLButtonElement | null;
}

function pointerEvent(type: string, init: { pointerType?: string; clientX?: number; button?: number; bubbles?: boolean }) {
  const event = new Event(type, { bubbles: init.bubbles ?? true });
  Object.assign(event, { pointerType: init.pointerType ?? "mouse", clientX: init.clientX ?? 0, button: init.button ?? 0 });
  return event;
}

it("あふれていないとき矢印ボタンを出さない（スマホ幅ではそもそも出ない）", async () => {
  await act(async () => renderChips());
  const group = container.querySelector('[role="radiogroup"]') as HTMLElement;
  mockScrollMetrics(group, { clientWidth: 1200, scrollWidth: 1200 });
  await act(async () => { window.dispatchEvent(new Event("resize")); });
  expect(byLabel("種目を左へスクロール")).toBeNull();
  expect(byLabel("種目を右へスクロール")).toBeNull();
  // 1列・折り返しなし・スクロールバーなし・右端フェードの #507 の決まりは維持する
  expect(group.className).toContain("flex-nowrap");
  expect(group.className).toContain("overflow-x-auto");
  expect(group.getAttribute("data-more-right")).toBe("false");
});

it("あふれているときだけ矢印ボタンを出す。スマホ幅では隠し、PC幅で見せる", async () => {
  await act(async () => renderChips());
  const group = container.querySelector('[role="radiogroup"]') as HTMLElement;
  mockScrollMetrics(group, { clientWidth: 300, scrollWidth: 1200 });
  await act(async () => { window.dispatchEvent(new Event("resize")); });
  const left = byLabel("種目を左へスクロール") as HTMLButtonElement;
  const right = byLabel("種目を右へスクロール") as HTMLButtonElement;
  expect(left).not.toBeNull();
  expect(right).not.toBeNull();
  // スマホ幅では出さず、md 以上（PC幅）で出す。既存の md ブレークポイントに揃える。
  for (const button of [left, right]) {
    expect(button.className).toContain("hidden");
    expect(button.className).toContain("md:inline-flex");
    expect(button.getAttribute("type")).toBe("button");
  }
  // 矢印ボタンはチップ列に被せず、横に並ぶ（チップの文字を隠さない）
  expect(left.compareDocumentPosition(group) & Node.DOCUMENT_POSITION_FOLLOWING).toBeTruthy();
  expect(group.compareDocumentPosition(right) & Node.DOCUMENT_POSITION_FOLLOWING).toBeTruthy();
});

it("左端では左ボタンが押せず、右端では右ボタンが押せない。押すたび見えている幅のおよそ1画面分動く", async () => {
  await act(async () => renderChips());
  const group = container.querySelector('[role="radiogroup"]') as HTMLElement;
  const { scrollBy, getLeft } = mockScrollMetrics(group, { clientWidth: 300, scrollWidth: 1200 });
  await act(async () => { window.dispatchEvent(new Event("resize")); });
  const left = byLabel("種目を左へスクロール") as HTMLButtonElement;
  const right = byLabel("種目を右へスクロール") as HTMLButtonElement;
  expect(left.disabled).toBe(true);
  expect(right.disabled).toBe(false);
  await act(async () => { right.click(); });
  expect(scrollBy).toHaveBeenCalledWith({ left: 240, behavior: "smooth" });
  expect(getLeft()).toBe(240);
  // 右端まで行くと右が押せなくなり、左が押せるようになる
  await act(async () => { group.dispatchEvent(new Event("scroll")); });
  expect((byLabel("種目を右へスクロール") as HTMLButtonElement).disabled).toBe(false);
  const metrics = mockScrollMetrics(group, { scrollLeft: 900, clientWidth: 300, scrollWidth: 1200 });
  await act(async () => { group.dispatchEvent(new Event("scroll")); });
  expect((byLabel("種目を右へスクロール") as HTMLButtonElement).disabled).toBe(true);
  expect((byLabel("種目を左へスクロール") as HTMLButtonElement).disabled).toBe(false);
  await act(async () => { (byLabel("種目を左へスクロール") as HTMLButtonElement).click(); });
  expect(metrics.scrollBy).toHaveBeenCalledWith({ left: -240, behavior: "smooth" });
});

it("マウスドラッグで横スクロールし、ドラッグしただけでは種目は変わらない", async () => {
  const onChange = vi.fn();
  await act(async () => renderChips(onChange));
  const group = container.querySelector('[role="radiogroup"]') as HTMLElement;
  mockScrollMetrics(group, { clientWidth: 300, scrollWidth: 1200 });
  await act(async () => { window.dispatchEvent(new Event("resize")); });
  await act(async () => { group.dispatchEvent(pointerEvent("pointerdown", { clientX: 200 })); });
  await act(async () => { window.dispatchEvent(pointerEvent("pointermove", { clientX: 100 })); });
  expect(group.scrollLeft).toBe(100);
  // ドラッグ中は掴んでいる見た目（grabbing相当）で、文字選択を付けない
  expect(group.className).toContain("md:cursor-grabbing");
  expect(group.className).toContain("select-none");
  await act(async () => { window.dispatchEvent(pointerEvent("pointerup", { clientX: 100 })); });
  expect(group.className).not.toContain("md:cursor-grabbing");
  const chip = group.querySelector('[data-chip="long-jump"]') as HTMLButtonElement;
  await act(async () => { chip.click(); });
  expect(onChange).not.toHaveBeenCalled();
});

it("小さい移動（タップ相当）では従来どおり選択が切り替わる", async () => {
  const onChange = vi.fn();
  await act(async () => renderChips(onChange));
  const group = container.querySelector('[role="radiogroup"]') as HTMLElement;
  mockScrollMetrics(group, { clientWidth: 300, scrollWidth: 1200 });
  await act(async () => { window.dispatchEvent(new Event("resize")); });
  await act(async () => { group.dispatchEvent(pointerEvent("pointerdown", { clientX: 200 })); });
  await act(async () => { window.dispatchEvent(pointerEvent("pointermove", { clientX: 203 })); });
  await act(async () => { window.dispatchEvent(pointerEvent("pointerup", { clientX: 203 })); });
  const chip = group.querySelector('[data-chip="long-jump"]') as HTMLButtonElement;
  await act(async () => { chip.click(); });
  expect(onChange).toHaveBeenCalledWith("long-jump");
});

it("タッチのスワイプはドラッグ実装で上書きせず、タップ選択も現行どおり", async () => {
  const onChange = vi.fn();
  await act(async () => renderChips(onChange));
  const group = container.querySelector('[role="radiogroup"]') as HTMLElement;
  mockScrollMetrics(group, { clientWidth: 300, scrollWidth: 1200 });
  await act(async () => { window.dispatchEvent(new Event("resize")); });
  await act(async () => { group.dispatchEvent(pointerEvent("pointerdown", { pointerType: "touch", clientX: 200 })); });
  await act(async () => { window.dispatchEvent(pointerEvent("pointermove", { pointerType: "touch", clientX: 50 })); });
  expect(group.scrollLeft).toBe(0);
  expect(group.className).not.toContain("md:cursor-grabbing");
  await act(async () => { window.dispatchEvent(pointerEvent("pointerup", { pointerType: "touch", clientX: 50 })); });
  const chip = group.querySelector('[data-chip="long-jump"]') as HTMLButtonElement;
  await act(async () => { chip.click(); });
  expect(onChange).toHaveBeenCalledWith("long-jump");
});

it("端で切れたチップを選ぶと全体が見える位置までスクロールし、見えているチップでは動かさない", async () => {
  const onChange = vi.fn();
  let value: CoachEventFilter = "all";
  const render = (next: CoachEventFilter) => {
    value = next;
    root.render(<CoachEventChips value={value} onChange={(v) => { onChange(v); render(v); }} />);
  };
  await act(async () => render("all"));
  const group = container.querySelector('[role="radiogroup"]') as HTMLElement;
  const { scrollTo } = mockScrollMetrics(group, { clientWidth: 300, scrollWidth: 1200 });
  // 右端で半分切れているチップ（1500m 相当：左端 280・幅 80 → 右端 360 が見えている幅 300 をはみ出す）
  const clipped = group.querySelector('[data-chip="1500m"]') as HTMLElement;
  Object.defineProperty(clipped, "offsetLeft", { value: 280, configurable: true });
  Object.defineProperty(clipped, "offsetWidth", { value: 80, configurable: true });
  await act(async () => { clipped.click(); });
  expect(onChange).toHaveBeenCalledWith("1500m");
  expect(scrollTo).toHaveBeenCalledWith({ left: 60, behavior: "smooth" });
  // すでに全体が見えているチップを選んでも列は動かさない
  scrollTo.mockClear();
  const visible = group.querySelector('[data-chip="100m"]') as HTMLElement;
  Object.defineProperty(visible, "offsetLeft", { value: 100, configurable: true });
  Object.defineProperty(visible, "offsetWidth", { value: 60, configurable: true });
  await act(async () => { visible.click(); });
  expect(onChange).toHaveBeenCalledWith("100m");
  expect(scrollTo).not.toHaveBeenCalled();
});
