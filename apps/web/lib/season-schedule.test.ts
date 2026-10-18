import { describe, expect, it } from "vitest";
import { isOngoing, isOver, seasonSchedule } from "./season-schedule";

const meet = (name: string, starts_on: string, ends_on: string | null = null) => ({
  name,
  starts_on: `${starts_on}T00:00:00Z`,
  ends_on: ends_on ? `${ends_on}T00:00:00Z` : null,
});

describe("season schedule", () => {
  const items = [
    meet("インターハイ", "2026-07-29", "2026-08-02"),
    meet("ニューイヤー駅伝", "2027-01-01"),
    meet("U20日本選手権", "2026-10-03", "2026-10-04"),
    meet("U18・U16", "2026-10-23", "2026-10-25"),
    meet("出雲駅伝", "2026-10-12"),
    meet("全日本大学駅伝", "2026-11-01"),
    meet("全中", "2026-08-20", "2026-08-23"),
  ];

  it("puts upcoming months first, starting with the current month", () => {
    const { upcoming } = seasonSchedule(items, "2026-10-15");
    expect(upcoming.map((g) => g.label)).toEqual(["10月", "11月", "1月"]);
    expect(upcoming[0].items.map((m) => m.name)).toEqual(["U18・U16"]);
  });

  it("moves finished meets of the current month to the finished list, latest month first", () => {
    const { ended } = seasonSchedule(items, "2026-10-15");
    expect(ended.map((g) => g.label)).toEqual(["10月", "8月", "7月"]);
    expect(ended[0].items.map((m) => m.name)).toEqual(["出雲駅伝", "U20日本選手権"]);
  });

  it("keeps a meet upcoming through its last day", () => {
    const interhigh = items[0];
    expect(isOver(interhigh, "2026-08-02")).toBe(false);
    expect(isOngoing(interhigh, "2026-08-02")).toBe(true);
    expect(isOver(interhigh, "2026-08-03")).toBe(true);
    expect(isOver(meet("one day", "2026-10-12"), "2026-10-12")).toBe(false);
  });

});
