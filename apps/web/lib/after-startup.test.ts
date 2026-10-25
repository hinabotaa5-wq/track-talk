/** @vitest-environment jsdom */
import { afterEach, expect, it, vi } from "vitest";
import { afterStartup } from "./after-startup";

afterEach(() => { vi.useRealTimers(); vi.restoreAllMocks(); vi.unstubAllGlobals(); });

it("waits for page load and a grace period before scheduling idle work", () => {
  vi.useFakeTimers();
  vi.spyOn(document, "readyState", "get").mockReturnValue("loading");
  const callback = vi.fn();
  const idle = vi.fn<(callback: () => void, options: { timeout: number }) => number>(() => 7);
  vi.stubGlobal("requestIdleCallback", idle);
  const cancel = afterStartup(callback);
  vi.advanceTimersByTime(10_000);
  expect(idle).not.toHaveBeenCalled();
  window.dispatchEvent(new Event("load"));
  vi.advanceTimersByTime(1_499);
  expect(idle).not.toHaveBeenCalled();
  vi.advanceTimersByTime(1);
  expect(idle).toHaveBeenCalledWith(callback, { timeout: 2_000 });
  idle.mock.calls[0][0]();
  expect(callback).toHaveBeenCalledOnce();
  vi.stubGlobal("cancelIdleCallback", vi.fn());
  cancel();
});

it("cancels pending work when leaving before the page loads", () => {
  vi.useFakeTimers();
  vi.spyOn(document, "readyState", "get").mockReturnValue("loading");
  const callback = vi.fn();
  afterStartup(callback)();
  window.dispatchEvent(new Event("load"));
  vi.runAllTimers();
  expect(callback).not.toHaveBeenCalled();
});

it("supports Safari without requestIdleCallback and cancels the timer", () => {
  vi.useFakeTimers();
  vi.spyOn(document, "readyState", "get").mockReturnValue("complete");
  vi.stubGlobal("requestIdleCallback", undefined);
  const callback = vi.fn();
  const cancel = afterStartup(callback);
  cancel();
  vi.runAllTimers();
  expect(callback).not.toHaveBeenCalled();
  afterStartup(callback);
  vi.advanceTimersByTime(1_500);
  expect(callback).toHaveBeenCalledOnce();
});

it("cancels an idle callback already queued", () => {
  vi.useFakeTimers();
  vi.spyOn(document, "readyState", "get").mockReturnValue("complete");
  vi.stubGlobal("requestIdleCallback", vi.fn(() => 42));
  const cancelIdle = vi.fn();
  vi.stubGlobal("cancelIdleCallback", cancelIdle);
  const cancel = afterStartup(vi.fn());
  vi.advanceTimersByTime(1_500);
  cancel();
  expect(cancelIdle).toHaveBeenCalledWith(42);
});
