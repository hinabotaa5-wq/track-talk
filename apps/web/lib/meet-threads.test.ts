import { describe, expect, it } from "vitest";
import { meetThreadForCompetition, meetThreadForMajor } from "./meet-threads";
import type { Competition, DomesticMajor, MeetThread } from "./types";

const thread = (over: Partial<MeetThread>): MeetThread => ({
  meet_key: "k",
  name: "大会",
  date_label: "2026年10月12日",
  post_id: "p",
  comment_count: 0,
  href: "/posts/p",
  now_running: [],
  major_slug: "",
  major_year: 0,
  competition_id: "",
  ...over,
});

describe("meet thread lookup for the competitions tab", () => {
  const meets = [
    thread({ post_id: "p1", major_slug: "izumo-ekiden", major_year: 2026 }),
    thread({ post_id: "p2", competition_id: "c1" }),
  ];

  it("finds the thread of an ongoing domestic major by slug and year", () => {
    const major = { slug: "izumo-ekiden", year: 2026 } as DomesticMajor;
    expect(meetThreadForMajor(meets, major)?.post_id).toBe("p1");
    expect(meetThreadForMajor(meets, { ...major, year: 2025 })).toBeUndefined();
  });

  it("finds the thread of an ongoing international meet by id", () => {
    expect(meetThreadForCompetition(meets, { id: "c1" } as Competition)?.post_id).toBe("p2");
    expect(meetThreadForCompetition(meets, { id: "c9" } as Competition)).toBeUndefined();
  });
});
