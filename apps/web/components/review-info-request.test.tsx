/** @vitest-environment jsdom */
import { act, StrictMode } from "react";
import { createRoot, type Root } from "react-dom/client";
import { afterEach, beforeEach, expect, it, vi } from "vitest";
import { SentBanner } from "./review-info-request";

let root: Root;
let container: HTMLDivElement;
beforeEach(() => {
  (globalThis as typeof globalThis & { IS_REACT_ACT_ENVIRONMENT: boolean }).IS_REACT_ACT_ENVIRONMENT = true;
  vi.useFakeTimers();
  container = document.createElement("div");
  document.body.appendChild(container);
  root = createRoot(container);
});
afterEach(async () => {
  await act(async () => root.unmount());
  container.remove();
  vi.restoreAllMocks();
  vi.useRealTimers();
});

it("再レンダーで3秒の期限を延ばさず、消去後に文言が変わっても復活しない", async () => {
  await act(async () => root.render(<StrictMode><SentBanner text="返答を送りました" /></StrictMode>));
  expect(vi.getTimerCount()).toBe(1);
  await act(async () => vi.advanceTimersByTime(2000));
  await act(async () => root.render(<StrictMode><SentBanner text="Response sent" /></StrictMode>));
  await act(async () => vi.advanceTimersByTime(999));
  expect(document.querySelector('[role="status"]')?.textContent).toBe("Response sent");
  await act(async () => vi.advanceTimersByTime(1));
  expect(document.querySelector('[role="status"]')).toBeNull();
  await act(async () => root.render(<StrictMode><SentBanner text="返答を送りました" /></StrictMode>));
  expect(document.querySelector('[role="status"]')).toBeNull();
});

it("3秒前のアンマウントでタイマーを破棄する", async () => {
  const clear = vi.spyOn(globalThis, "clearTimeout");
  await act(async () => root.render(<SentBanner text="差し戻しました" />));
  expect(vi.getTimerCount()).toBe(1);
  await act(async () => root.render(null));
  expect(clear).toHaveBeenCalledTimes(1);
  expect(vi.getTimerCount()).toBe(0);
  await act(async () => vi.advanceTimersByTime(3000));
  expect(document.querySelector('[role="status"]')).toBeNull();
});
