/** @vitest-environment jsdom */
import { act } from "react";
import { createRoot, type Root } from "react-dom/client";
import { afterEach, beforeEach, expect, it, vi } from "vitest";
import { HorizontalFeed } from "./horizontal-feed";

let root: Root;
let container: HTMLDivElement;
let desktop = true;
beforeEach(async () => {
  (globalThis as typeof globalThis & { IS_REACT_ACT_ENVIRONMENT: boolean }).IS_REACT_ACT_ENVIRONMENT = true;
  desktop = true;
  vi.stubGlobal("matchMedia", () => ({ matches: desktop }));
  container = document.createElement("div");
  document.body.appendChild(container);
  root = createRoot(container);
  await act(async () => root.render(<HorizontalFeed><a href="/posts/1">投稿1</a><a href="/posts/2">投稿2</a></HorizontalFeed>));
});
afterEach(async () => {
  await act(async () => root.unmount());
  container.remove();
  vi.unstubAllGlobals();
});
function track() {
  const el = container.querySelector<HTMLElement>('[role="region"]')!;
  Object.defineProperties(el, { clientWidth: { value: 400, configurable: true }, scrollWidth: { value: 1200, configurable: true } });
  return el;
}
function wheel(el: HTMLElement, options: WheelEventInit) {
  const event = new WheelEvent("wheel", { bubbles: true, cancelable: true, ...options });
  el.dispatchEvent(event);
  return event;
}
it("PCは下ホイールで次へ、上ホイールで前へ進む", () => {
  const el = track();
  expect(wheel(el, { deltaY: 240 }).defaultPrevented).toBe(true);
  expect(el.scrollLeft).toBe(240);
  wheel(el, { deltaY: -120 });
  expect(el.scrollLeft).toBe(120);
  wheel(el, { deltaY: 100, deltaMode: WheelEvent.DOM_DELTA_PAGE });
  expect(el.scrollLeft).toBe(800);
});
it("端ではページスクロールに戻り、横操作と拡大操作を妨げない", () => {
  const el = track();
  expect(wheel(el, { deltaY: -120 }).defaultPrevented).toBe(false);
  expect(wheel(el, { deltaX: 200 }).defaultPrevented).toBe(false);
  expect(wheel(el, { deltaY: 120, ctrlKey: true }).defaultPrevented).toBe(false);
  el.scrollLeft = 800;
  expect(wheel(el, { deltaY: 120 }).defaultPrevented).toBe(false);
  desktop = false;
  expect(wheel(el, { deltaY: -120 }).defaultPrevented).toBe(false);
});
it("スワイプの案内と左右の矢印を表示しない", () => {
  expect(container.textContent).not.toContain("スワイプ");
  expect(container.textContent).not.toContain("ホイール");
  expect(container.querySelector("button")).toBeNull();
  expect(container.querySelector('[aria-describedby]')).toBeNull();
});
