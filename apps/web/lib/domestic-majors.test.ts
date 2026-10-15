import { describe, expect, it } from "vitest";
import { undecidedDomesticMeets } from "./domestic-majors";
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
  it("lists the meets that have no edition this season as undecided", () => {
    expect(undecidedDomesticMeets([])).toHaveLength(32);
    const undecided = undecidedDomesticMeets([zenchu]);
    expect(undecided).toHaveLength(31);
    expect(undecided.some((meet) => meet.slug === "zenchu")).toBe(false);
    expect(undecided.some((meet) => meet.slug === "new-year-ekiden")).toBe(true);
  });
});
