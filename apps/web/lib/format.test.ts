import { describe, expect, it } from "vitest";
import { fromJstInput, toJstInput, todayInTokyo } from "./format";

describe("todayInTokyo", () => {
  it("reads today in Japan time", () => {
    expect(todayInTokyo(new Date("2026-10-15T15:30:00Z"))).toBe("2026-10-16");
    expect(todayInTokyo(new Date("2026-10-15T14:59:00Z"))).toBe("2026-10-15");
  });
});

describe("datetime-local in Japan time", () => {
  it("writes and reads the wall clock in Japan time regardless of the device", () => {
    expect(toJstInput("2026-10-20T09:00:00.000Z")).toBe("2026-10-20T18:00");
    expect(toJstInput("2026-10-19T15:30:00.000Z")).toBe("2026-10-20T00:30");
    expect(fromJstInput("2026-10-20T18:00").toISOString()).toBe("2026-10-20T09:00:00.000Z");
    expect(fromJstInput(toJstInput("2026-12-31T14:59:00.000Z")).toISOString()).toBe("2026-12-31T14:59:00.000Z");
  });

  it("rejects values it cannot read", () => {
    expect(toJstInput("not a date")).toBe("");
    expect(Number.isNaN(fromJstInput("").getTime())).toBe(true);
    expect(Number.isNaN(fromJstInput("2026-10-20").getTime())).toBe(true);
  });
});
