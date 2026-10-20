/** @vitest-environment jsdom */
import { afterEach, describe, expect, it, vi } from "vitest";
import { videoDuration } from "./review";

afterEach(() => {
  vi.useRealTimers();
  vi.unstubAllGlobals();
});

describe("videoDuration", () => {
  it("gives up when the browser never reports the metadata", async () => {
    vi.useFakeTimers();
    vi.stubGlobal("URL", Object.assign(URL, { createObjectURL: () => "blob:x", revokeObjectURL: () => {} }));
    HTMLMediaElement.prototype.load = () => {};
    const pending = videoDuration(new File(["x"], "a.mov"));
    await vi.advanceTimersByTimeAsync(10_001);
    await expect(pending).resolves.toBeNull();
  });
});
