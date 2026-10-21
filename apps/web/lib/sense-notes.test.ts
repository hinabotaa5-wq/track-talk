import { describe, expect, it } from "vitest";
import {
  groupSenseNotesByDay,
  senseCheer,
  senseDayLabel,
  senseJarLevel,
  senseJarSize,
  senseSummaryMinNotes,
  sortSenseNotes,
  senseSummaryRangeLabel,
  type SenseNote,
} from "./sense-notes";

function note(id: string, day: string): SenseNote {
  return { id, body: id, event: null, event_label: null, day, created_at: `${day}T10:00:00Z`, updated_at: `${day}T10:00:00Z` };
}

describe("sense notes", () => {
  it("fills one jar every 6 notes", () => {
    expect(senseJarLevel(0)).toEqual({ filled: 0, current: 0 });
    expect(senseJarLevel(5)).toEqual({ filled: 0, current: 5 });
    expect(senseJarLevel(6)).toEqual({ filled: 1, current: 0 });
    expect(senseJarLevel(7)).toEqual({ filled: 1, current: 1 });
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
    expect(senseCheer({ total: 6, days: 6, streak: 6, today: 1 }).body).toContain("1杯目");
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

  it("sorts notes by day, newest first, even when an old day was written last", () => {
    const notes = [note("a", "2026-10-16"), { ...note("b", "2026-10-10"), created_at: "2026-10-17T10:00:00Z" }, note("c", "2026-10-14")];
    expect(sortSenseNotes(notes).map((n) => n.id)).toEqual(["a", "c", "b"]);
  });

  it("labels the range of a summary", () => {
    // 2026-10-20 10:00 JST
    const now = new Date("2026-10-20T01:00:00Z");
    expect(senseSummaryRangeLabel({ from_day: "2026-10-14", to_day: "2026-10-20" }, now)).toBe("10月14日(水)〜10月20日(火)");
  });
});
