import { describe, expect, it } from "vitest";
import { todayInTokyo } from "./format";

describe("todayInTokyo", () => {
  it("reads today in Japan time", () => {
    expect(todayInTokyo(new Date("2026-10-15T15:30:00Z"))).toBe("2026-10-16");
    expect(todayInTokyo(new Date("2026-10-15T14:59:00Z"))).toBe("2026-10-15");
  });
});
