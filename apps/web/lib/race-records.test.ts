import { describe, expect, it } from "vitest";
import {
  formatRaceMark,
  formatRaceWind,
  parseRaceMark,
  parseRaceWind,
  toggleRaceWindSign,
  raceChartBox,
  raceChartPoints,
  raceChartDomain,
  sortRaceRecords,
  raceEventHasWind,
  raceMarkInput,
  raceMarkKind,
  raceRecordEvents,
  sortRaceEvents,
  type RaceRecord,
} from "./race-records";

function record(raced_on: string, mark: number, extra: Partial<RaceRecord> = {}): RaceRecord {
  return { id: `${raced_on}-${mark}`, raced_on, event: "100m", event_label: "100m", mark, wind: null, wind_aided: false, is_pb: false, memo: "", meet_name: "", created_at: "", ...extra };
}

describe("race records", () => {
  it("knows each event's unit and wind", () => {
    expect(raceMarkKind("100m")).toBe("time");
    expect(raceMarkKind("4x400")).toBe("time");
    expect(raceMarkKind("long-jump")).toBe("distance");
    expect(raceMarkKind("decathlon")).toBe("points");
    expect(raceEventHasWind("200m")).toBe(true);
    expect(raceEventHasWind("400m")).toBe(false);
    expect(raceRecordEvents.map((item) => item.value as string)).not.toContain("other");
  });

  it("reads marks as they are written", () => {
    expect(parseRaceMark("10.85", "time")).toBe(1085);
    expect(parseRaceMark("１０．８５", "time")).toBe(1085);
    expect(parseRaceMark("1:58.32", "time")).toBe(11832);
    expect(parseRaceMark("15:02.1", "time")).toBe(90210);
    expect(parseRaceMark("49.5秒", "time")).toBe(4950);
    expect(parseRaceMark("1:5.32", "time")).toBeNull();
    expect(parseRaceMark("1:60.00", "time")).toBeNull();
    expect(parseRaceMark("abc", "time")).toBeNull();
    expect(parseRaceMark("0", "time")).toBeNull();
    expect(parseRaceMark("7.05", "distance")).toBe(705);
    expect(parseRaceMark("7.5m", "distance")).toBe(750);
    expect(parseRaceMark("60", "distance")).toBe(6000);
    expect(parseRaceMark("7.055", "distance")).toBeNull();
    expect(parseRaceMark("5,800", "points")).toBe(5800);
    expect(parseRaceMark("58.5", "points")).toBeNull();
  });

  it("shows marks the way athletes read them", () => {
    expect(formatRaceMark(1085, "time")).toBe("10.85");
    expect(formatRaceMark(11832, "time")).toBe("1:58.32");
    expect(formatRaceMark(90205, "time")).toBe("15:02.05");
    expect(formatRaceMark(705, "distance")).toBe("7.05m");
    expect(formatRaceMark(5800, "points")).toBe("5,800点");
    expect(raceMarkInput(705, "distance")).toBe("7.05");
    expect(raceMarkInput(5800, "points")).toBe("5800");
    expect(parseRaceMark(raceMarkInput(11832, "time"), "time")).toBe(11832);
  });

  it("reads and shows wind in tenths", () => {
    expect(parseRaceWind("+1.8")).toBe(18);
    expect(parseRaceWind("－0.5")).toBe(-5);
    expect(parseRaceWind("2")).toBe(20);
    expect(parseRaceWind("")).toBeUndefined();
    expect(parseRaceWind("10.0")).toBeNull();
    expect(formatRaceWind(18)).toBe("+1.8");
    expect(formatRaceWind(-5)).toBe("-0.5");
    expect(formatRaceWind(0)).toBe("±0.0");
  });

  it("flips the wind sign for number-only keyboards", () => {
    expect(toggleRaceWindSign("1.8")).toBe("-1.8");
    expect(toggleRaceWindSign("-1.8")).toBe("+1.8");
    expect(toggleRaceWindSign("+1.8")).toBe("-1.8");
    expect(toggleRaceWindSign("－0.5")).toBe("+0.5");
    expect(toggleRaceWindSign("")).toBe("-");
    expect(parseRaceWind(toggleRaceWindSign(toggleRaceWindSign("2.0")))).toBe(20);
  });

  it("orders events like the practice list", () => {
    const sorted = sortRaceEvents([
      { event: "shot-put", event_label: "砲丸投", kind: "distance", wind: false, count: 1 },
      { event: "100m", event_label: "100m", kind: "time", wind: true, count: 2 },
    ]);
    expect(sorted.map((item) => item.event)).toEqual(["100m", "shot-put"]);
  });

  it("puts better marks higher, with time events upside down", () => {
    const box = raceChartBox;
    const bottom = box.height - box.bottom;
    const track = raceChartPoints([record("2026-05-01", 1120), record("2026-06-01", 1080), record("2026-07-01", 1100)], "time");
    expect(track[0].x).toBe(box.left);
    expect(track[2].x).toBe(box.width - box.right);
    // 速い(小さい)ほど上。
    expect(track[1].y).toBeGreaterThan(box.top);
    expect(track[0].y).toBeLessThan(bottom);
    expect(track[2].y).toBeCloseTo((box.top + bottom) / 2);

    const field = raceChartPoints([record("2026-05-01", 650), record("2026-06-01", 705)], "distance");
    expect(field[1].y).toBeGreaterThan(box.top);
    expect(field[0].y).toBeLessThan(bottom);
  });

  it("centres a single record", () => {
    const box = raceChartBox;
    const [only] = raceChartPoints([record("2026-05-01", 1100)], "time");
    expect(only.x).toBe(box.left + (box.width - box.left - box.right) / 2);
    expect(only.y).toBe(box.top + (box.height - box.top - box.bottom) / 2);
    expect(raceChartPoints([], "time")).toEqual([]);
  });
});

it("小さなタイム差をグラフいっぱいに拡大しない", () => {
  const records = [record("2026-05-01", 1100), record("2026-06-01", 1101)];
  const points = raceChartPoints(records, "time");
  expect(Math.abs(points[0].y - points[1].y)).toBeLessThan(1);
  const domain = raceChartDomain(records.map((r) => r.mark));
  expect(domain.high - domain.low).toBeGreaterThanOrEqual(220);
});
it("同日に逆順に入力しても予選から決勝へ別の位置で並ぶ", () => {
  const rows = [record("2026-05-01", 1080, { id: "final", round: "決勝" }), record("2026-05-01", 1100, { id: "heat", round: "予選" }), record("2026-05-01", 1090, { id: "semi", round: "準決勝" })];
  const points = raceChartPoints(rows, "time");
  expect(points.map((p) => p.record.id)).toEqual(["heat", "semi", "final"]);
  expect(points[0].x).toBeLessThan(points[1].x);
  expect(points[1].x).toBeLessThan(points[2].x);
  expect(sortRaceRecords(rows).map((r) => r.round)).toEqual(["予選", "準決勝", "決勝"]);
});
