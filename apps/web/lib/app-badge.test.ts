import { afterEach, describe, expect, it, vi } from "vitest";
import { applyAppBadge, clearAppBadge } from "./app-badge";

afterEach(() => {
  vi.unstubAllGlobals();
});

function stubNavigator(badge: { setAppBadge?: (...args: never[]) => unknown; clearAppBadge?: (...args: never[]) => unknown }) {
  vi.stubGlobal("navigator", badge);
}

describe("applyAppBadge", () => {
  it("sets a safe positive integer", async () => {
    const setAppBadge = vi.fn(async () => undefined);
    stubNavigator({ setAppBadge });
    await applyAppBadge(3);
    expect(setAppBadge).toHaveBeenCalledWith(3);
  });

  it("clears the badge for zero", async () => {
    const clearAppBadge = vi.fn(async () => undefined);
    stubNavigator({ clearAppBadge });
    await applyAppBadge(0);
    expect(clearAppBadge).toHaveBeenCalledTimes(1);
  });

  it("does not call anything for invalid values", async () => {
    const setAppBadge = vi.fn(async () => undefined);
    const clearAppBadge = vi.fn(async () => undefined);
    stubNavigator({ setAppBadge, clearAppBadge });
    for (const value of [null, undefined, -1, 0.5, Number.NaN, "3", Number.POSITIVE_INFINITY, Number.MAX_SAFE_INTEGER + 1]) {
      await applyAppBadge(value);
    }
    expect(setAppBadge).not.toHaveBeenCalled();
    expect(clearAppBadge).not.toHaveBeenCalled();
  });

  it("does nothing harmful when the API is missing", async () => {
    stubNavigator({});
    await expect(applyAppBadge(3)).resolves.toBeUndefined();
    await expect(applyAppBadge(0)).resolves.toBeUndefined();
  });

  it("does not let a badge failure break the caller", async () => {
    stubNavigator({ setAppBadge: vi.fn(async () => { throw new Error("denied"); }) });
    await expect(applyAppBadge(3)).resolves.toBeUndefined();
    stubNavigator({ clearAppBadge: vi.fn(async () => { throw new Error("denied"); }) });
    await expect(applyAppBadge(0)).resolves.toBeUndefined();
  });
});

describe("clearAppBadge", () => {
  it("clears when supported and never throws", async () => {
    const clear = vi.fn(async () => undefined);
    stubNavigator({ clearAppBadge: clear });
    await expect(clearAppBadge()).resolves.toBeUndefined();
    expect(clear).toHaveBeenCalledTimes(1);
    stubNavigator({});
    await expect(clearAppBadge()).resolves.toBeUndefined();
  });
});
