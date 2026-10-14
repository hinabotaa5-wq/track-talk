import { describe, expect, it } from "vitest";
import {
  DOMESTIC_CATEGORIES,
  domesticMeetCards,
  domesticMeetsOf,
  isDomesticMajorOver,
} from "./domestic-majors";
import type { DomesticMajor } from "./types";

const zenchu: DomesticMajor = {
  slug: "zenchu",
  year: 2026,
  name: "第53回全日本中学校陸上競技選手権大会",
  category: "youth",
  venue: "山口・維新百年記念公園陸上競技場",
  starts_on: "2026-08-20T00:00:00Z",
  ends_on: "2026-08-23T00:00:00Z",
  schedule_label: "2026年8月20日〜23日",
  site_url: "https://tf.zenchuu.jp/",
};

describe("domestic majors", () => {
  it("always lists all sixteen meets and fills in the ones the API returned", () => {
    const cards = domesticMeetCards([zenchu]);
    expect(cards).toHaveLength(16);
    expect(cards.find((card) => card.slug === "zenchu")?.major).toBe(zenchu);
    expect(cards.find((card) => card.slug === "incare-kanto")?.major).toBeNull();
  });

  it("ignores a slug the page does not know", () => {
    expect(domesticMeetCards([{ ...zenchu, slug: "unknown" }]).every((card) => card.major === null)).toBe(true);
  });

  it("groups the cards by category in page order", () => {
    const cards = domesticMeetCards([]);
    expect(DOMESTIC_CATEGORIES.map((c) => domesticMeetsOf(cards, c.key).length)).toEqual([5, 9, 2]);
    expect(domesticMeetsOf(cards, "student").at(-1)?.slug).toBe("nihon-incare");
  });

  it("treats a meet as over only after its last day", () => {
    expect(isDomesticMajorOver(zenchu, "2026-08-23")).toBe(false);
    expect(isDomesticMajorOver(zenchu, "2026-08-24")).toBe(true);
  });
});
