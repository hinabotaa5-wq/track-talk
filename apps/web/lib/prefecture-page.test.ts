import { describe, expect, it } from "vitest";
import { isPrefecturePageCode, prefectureCompetitionsHref, prefecturePageHref, prefectureRecruitmentsHref } from "./prefecture-page";

describe("prefecture page", () => {
  it("has a page for each of the 47 prefectures only", () => {
    expect(isPrefecturePageCode("01")).toBe(true);
    expect(isPrefecturePageCode("47")).toBe(true);
    expect(isPrefecturePageCode("00")).toBe(false);
    expect(isPrefecturePageCode("48")).toBe(false);
    expect(prefecturePageHref("39")).toBe("/prefectures/39");
  });

  it("links to the existing lists", () => {
    expect(prefectureCompetitionsHref("39", "2026-11-03T00:00:00Z")).toBe("/competitions/regional?prefecture=39&month=11");
    expect(prefectureCompetitionsHref("39")).toBe("/competitions/regional?prefecture=39");
    expect(prefectureRecruitmentsHref("高知県")).toBe(`/?sort=recruitment&q=${encodeURIComponent("高知県")}`);
  });
});
