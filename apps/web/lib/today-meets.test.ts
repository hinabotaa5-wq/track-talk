import { describe, expect, it } from "vitest";
import { balloonMessage, isFeaturedInternational, todayMeets } from "./today-meets";
import type { Competition, DomesticMajor } from "./types";

const izumo: DomesticMajor = {
  slug: "izumo-ekiden",
  year: 2026,
  name: "第38回出雲全日本大学選抜駅伝競走",
  category: "distance",
  venue: "島根・出雲大社正面鳥居前",
  starts_on: "2026-10-12T00:00:00Z",
  ends_on: "2026-10-12T00:00:00Z",
  schedule_label: "2026年10月12日",
  site_url: "https://www.izumo-ekiden.jp/",
};

function international(overrides: Partial<Competition>): Competition {
  return {
    id: "c1",
    name: "コンチネンタルツアー・ゴールド",
    starts_on: "2026-10-11T00:00:00Z",
    ends_on: "2026-10-13T00:00:00Z",
    location: "",
    prefecture: "00",
    season_year: 2026,
    organizer_url: "https://worldathletics.org/competitions/x",
    source_url: "https://worldathletics.org/competition/calendar-results",
    link_label: "大会ページ",
    guidelines_url: "",
    ...overrides,
  };
}

describe("today meets", () => {
  it("picks only the featured international series", () => {
    expect(isFeaturedInternational(international({ name: "オリンピック・ロサンゼルス" }))).toBe(true);
    expect(isFeaturedInternational(international({ name: "世界陸上・北京" }))).toBe(true);
    expect(isFeaturedInternational(international({ name: "U20世界陸上・ユージーン" }))).toBe(true);
    expect(isFeaturedInternational(international({ name: "ダイヤモンドリーグ・ファイナル" }))).toBe(true);
    expect(isFeaturedInternational(international({ name: "第21回アジア競技大会" }))).toBe(true);
    expect(
      isFeaturedInternational(international({ name: "ゴールデンガラ", organizer_url: "http://rome.diamondleague.com/" })),
    ).toBe(true);
    expect(isFeaturedInternational(international({}))).toBe(false);
    expect(isFeaturedInternational(international({ name: "世界室内陸上" }))).toBe(false);
    expect(isFeaturedInternational(international({ name: "U18世界陸上" }))).toBe(false);
    expect(isFeaturedInternational(international({ name: "冬季オリンピック" }))).toBe(false);
  });

  it("lists the meets held today with their pages", () => {
    const dl = international({ id: "dl", name: "ゴールデンガラ", organizer_url: "http://rome.diamondleague.com/" });
    const tour = international({ id: "tour" });
    expect(todayMeets([izumo], [dl, tour], "2026-10-12")).toEqual([
      { key: "izumo-ekiden:2026", name: izumo.name, href: "https://www.izumo-ekiden.jp/" },
      { key: "dl", name: "ゴールデンガラ", href: "http://rome.diamondleague.com/" },
    ]);
    expect(todayMeets([izumo], [dl], "2026-10-13").map((meet) => meet.key)).toEqual(["dl"]);
    expect(todayMeets([izumo], [dl], "2026-10-14")).toEqual([]);
  });

  it("falls back to the competitions tab when a meet has no page", () => {
    expect(todayMeets([{ ...izumo, site_url: "" }], [], "2026-10-12")[0].href).toBe("/competitions");
  });

  it("names every meet in the message", () => {
    expect(balloonMessage([{ key: "a", name: "世界陸上・北京", href: "" }])).toBe("今日は世界陸上・北京が開催されてるよ！");
    expect(
      balloonMessage([
        { key: "a", name: "A", href: "" },
        { key: "b", name: "B", href: "" },
      ]),
    ).toBe("今日はA、Bが開催されてるよ！");
  });
});
