import { renderToStaticMarkup } from "react-dom/server";
import { describe, expect, it } from "vitest";
import type { InjuryProgress } from "@/lib/injury";
import { InjuryProgressCompareView } from "./injury-progress-compare";

const progress: InjuryProgress = {
  min_sample: 5,
  posts: [
    {
      post_id: "p1",
      body_part: "hamstring-strain",
      body_part_label: "ハムストリングの肉離れ",
      posted_on: "2026-09-01",
      stages: [
        { stage: "resting", stage_label: "安静中", my_days: 0, median_days: null },
        { stage: "rehab", stage_label: "リハビリ中", my_days: 12, median_days: 14 },
        { stage: "jogging", stage_label: "ジョグ再開", my_days: null, median_days: 30 },
        { stage: "practice", stage_label: "練習に復帰", my_days: null, median_days: null },
        { stage: "race", stage_label: "大会に復帰", my_days: null, median_days: 55 },
      ],
    },
  ],
};

function row(html: string, label: string) {
  const match = new RegExp(`<tr><th scope="row"[^>]*>${label}</th>(.*?)</tr>`).exec(html);
  if (!match) throw new Error(`row ${label} not found`);
  return match[1].replace(/<[^>]+>/g, "|");
}

describe("InjuryProgressCompareView", () => {
  it("shows my days next to everyone's median, stage by stage", () => {
    const html = renderToStaticMarkup(<InjuryProgressCompareView progress={progress} />);
    expect(html).toContain("みんなの目安との比較");
    expect(row(html, "安静中")).toContain("投稿日（0日目）");
    expect(row(html, "リハビリ中")).toContain("12日目に到達");
    expect(row(html, "リハビリ中")).toContain("14日");
    expect(row(html, "ジョグ再開")).toContain("まだ");
    expect(row(html, "大会に復帰")).toContain("55日");
  });

  it("does not show a number for a stage below the sample minimum", () => {
    const html = renderToStaticMarkup(<InjuryProgressCompareView progress={progress} />);
    const practice = row(html, "練習に復帰");
    expect(practice).toContain("5人未満のため非表示");
    expect(practice).not.toMatch(/\d+日/);
  });

  it("renders nothing without injury posts", () => {
    expect(renderToStaticMarkup(<InjuryProgressCompareView progress={{ min_sample: 5, posts: [] }} />)).toBe("");
    expect(renderToStaticMarkup(<InjuryProgressCompareView progress={null} />)).toBe("");
  });
});
