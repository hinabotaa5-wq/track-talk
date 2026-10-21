import { describe, expect, it } from "vitest";
import {
  groupSenseNotesByDay,
  senseCheer,
  senseDayLabel,
  senseJarLevel,
  senseJarSize,
  senseSummaryMinNotes,
  senseSummaryNextLabel,
  senseSummaryRangeLabel,
  type SenseNote,
} from "./sense-notes";

function note(id: string, day: string): SenseNote {
  return { id, body: id, event: null, event_label: null, day, created_at: `${day}T10:00:00Z`, updated_at: `${day}T10:00:00Z` };
}

describe("sense notes", () => {
  it("fills one jar every 9 notes", () => {
    expect(senseJarLevel(0)).toEqual({ filled: 0, current: 0 });
    expect(senseJarLevel(8)).toEqual({ filled: 0, current: 8 });
    expect(senseJarLevel(9)).toEqual({ filled: 1, current: 0 });
    expect(senseJarLevel(10)).toEqual({ filled: 1, current: 1 });
  });

  it("lets a summary be made when the jar is full", () => {
    expect(senseSummaryMinNotes).toBe(senseJarSize);
  });

  it("always opens the cheer with 今日も頑張ったね", () => {
    const first = senseCheer({ total: 1, days: 1, streak: 1, today: 1 });
    expect(first.title).toBe("今日も頑張ったね");
    expect(first.body).toContain("はじめて");
    expect(senseCheer({ total: 5, days: 3, streak: 3, today: 1 }).body).toContain("3日続けて");
    expect(senseCheer({ total: 5, days: 3, streak: 1, today: 2 }).body).toContain("2つ目");
    expect(senseCheer({ total: 9, days: 9, streak: 9, today: 1 }).body).toContain("1杯目");
  });

  it("groups notes by day keeping the order", () => {
    const groups = groupSenseNotesByDay([note("a", "2026-10-16"), note("b", "2026-10-16"), note("c", "2026-10-14")]);
    expect(groups.map((g) => [g.day, g.notes.map((n) => n.id)])).toEqual([
      ["2026-10-16", ["a", "b"]],
      ["2026-10-14", ["c"]],
    ]);
  });

  it("labels days with the weekday and the year only for other years", () => {
    const now = new Date("2026-10-16T03:00:00Z");
    expect(senseDayLabel("2026-10-16", now)).toBe("10月16日(金)");
    expect(senseDayLabel("2025-12-31", now)).toBe("2025年12月31日(水)");
  });

  it("labels the week of a summary and the day the next one opens", () => {
    // 2026-10-20 10:00 JST
    const now = new Date("2026-10-20T01:00:00Z");
    expect(senseSummaryRangeLabel({ from_day: "2026-10-14", to_day: "2026-10-20" }, now)).toBe("10月14日(水)〜10月20日(火)");
    expect(senseSummaryNextLabel("2026-10-27T01:00:00Z", now)).toBe("10月27日(火)");
    expect(senseSummaryNextLabel(null, now)).toBeNull();
  });
});
