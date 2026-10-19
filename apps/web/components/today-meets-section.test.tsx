import { renderToStaticMarkup } from "react-dom/server";
import { describe, expect, it } from "vitest";
import type { MeetThread } from "@/lib/types";
import { TodayMeetsSection } from "./today-meets-section";

const meets: MeetThread[] = [
  {
    meet_key: "m:izumo-ekiden:2026",
    name: "出雲駅伝",
    date_label: "2026年10月12日",
    post_id: "p1",
    comment_count: 3,
    href: "/posts/p1",
    now_running: ["女子100m予選", "男子走幅跳決勝"],
  },
  {
    meet_key: "c:00:2026:ダイヤモンドリーグ・ファイナル:2026-10-11",
    name: "ダイヤモンドリーグ・ファイナル",
    date_label: "2026年10月11日",
    post_id: "p2",
    comment_count: 0,
    href: "/posts/p2",
    now_running: [],
  },
];

describe("TodayMeetsSection", () => {
  it("renders nothing when there are no meets today", () => {
    expect(renderToStaticMarkup(<TodayMeetsSection meets={[]} />)).toBe("");
  });

  it("lists each meet's thread with its date and comment count", () => {
    const html = renderToStaticMarkup(<TodayMeetsSection meets={meets} />);
    expect(html).toContain("今日開催中");
    expect(html).toContain('href="/posts/p1"');
    expect(html).toContain('href="/posts/p2"');
    expect(html).toContain("出雲駅伝");
    expect(html).toContain("2026年10月12日");
    expect(html).toContain("ダイヤモンドリーグ・ファイナル");
    // コメント数(3 と 0)が出る。
    expect(html).toContain(">3<");
  });

  it("shows what is running now only for meets with a timetable", () => {
    const html = renderToStaticMarkup(<TodayMeetsSection meets={meets} />);
    expect(html).toContain("いま実施中: 女子100m予選、男子走幅跳決勝");
    expect(html.match(/いま実施中/g)).toHaveLength(1);
  });
});
