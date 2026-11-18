import { describe, expect, it } from "vitest";
import { formatChatDateTime, formatDateTime, fromJstInput, toJstInput, todayInTokyo } from "./format";

it("チャット日時は日本時間の月日・時分だけで、年と秒を省く", () => {
  expect(formatChatDateTime("2026-11-17T21:19:37Z")).toBe("11/18 6:19");
  expect(formatChatDateTime("2026-12-31T15:05:00Z")).toBe("1/1 0:05");
  expect(formatChatDateTime("2026-01-02T12:34:00+09:00")).toBe("1/2 12:34");
  expect(formatChatDateTime("invalid")).toBe("");
  expect(formatDateTime("2026-11-17T21:19:37Z")).toContain("2026");
});

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
