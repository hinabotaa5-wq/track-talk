import { describe, expect, it } from "vitest";
import { formatRemaining, remainingParts } from "./menu-contest";

describe("remainingParts", () => {
  const now = Date.parse("2026-11-03T07:00:00+09:00");

  it("splits the time into days, hours and minutes", () => {
    const until = Date.parse("2026-11-12T00:00:00+09:00");
    const parts = remainingParts(until, now);
    expect(parts).toMatchObject({ days: 8, hours: 17, minutes: 0, urgent: false });
    expect(formatRemaining(parts!)).toBe("8日 17時間 0分");
  });

  it("rounds up the last seconds and marks the last day", () => {
    const parts = remainingParts(now + 30_000, now);
    expect(parts).toMatchObject({ days: 0, hours: 0, minutes: 1, urgent: true });
    expect(formatRemaining(parts!)).toBe("1分");
    expect(formatRemaining(remainingParts(now + (5 * 60 + 2) * 60_000, now)!)).toBe("5時間 2分");
  });

  it("returns null once the deadline has passed", () => {
    expect(remainingParts(now, now)).toBeNull();
    expect(remainingParts(now - 1, now)).toBeNull();
  });
});
