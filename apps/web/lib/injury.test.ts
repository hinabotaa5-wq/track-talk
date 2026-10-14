import { describe, expect, it } from "vitest";
import { injuryFeedPath, injuryHistoryStageText, injuryRepeatedParts, injuryStageStatText, injuryTypicalStages, isInjuryBodyPart, isInjuryMood, isInjuryStage } from "./injury";

describe("injury", () => {
  it("builds the feed path with only valid filters", () => {
    expect(injuryFeedPath("", "", 1)).toBe("/v1/posts?kind=injury&sort=new&page=1&per_page=20");
    expect(injuryFeedPath("vent", "knee", 2)).toBe(
      "/v1/posts?kind=injury&sort=new&page=2&per_page=20&mood=vent&part=knee",
    );
    expect(injuryFeedPath("angry", "head", 1)).toBe("/v1/posts?kind=injury&sort=new&page=1&per_page=20");
  });

  it("adds the latest recovery stage filter only when valid", () => {
    expect(injuryFeedPath("", "", 1, "race")).toBe("/v1/posts?kind=injury&sort=new&page=1&per_page=20&stage=race");
    expect(injuryFeedPath("", "", 1, "flying")).toBe("/v1/posts?kind=injury&sort=new&page=1&per_page=20");
  });

  it("validates mood and body part codes", () => {
    expect(isInjuryMood("rehab")).toBe(true);
    expect(isInjuryMood("other")).toBe(false);
    expect(isInjuryBodyPart("lower-back")).toBe(true);
    expect(isInjuryBodyPart(null)).toBe(false);
  });

  it("validates recovery stage codes", () => {
    expect(isInjuryStage("jogging")).toBe(true);
    expect(isInjuryStage("vent")).toBe(false);
    expect(isInjuryStage(undefined)).toBe(false);
  });

  it("describes a stage's median, or how many more reports are needed", () => {
    const base = { stage: "jogging", stage_label: "ジョグ再開" } as const;
    expect(injuryStageStatText({ ...base, reached: 6, median_days: 42 }, 5)).toBe("投稿から約42日（6人）");
    expect(injuryStageStatText({ ...base, reached: 5, median_days: 0 }, 5)).toBe("投稿から当日（5人）");
    expect(injuryStageStatText({ ...base, reached: 3, median_days: null }, 5)).toBe("報告 3人（あと2人で目安を表示）");
  });

  it("picks only the stages with a median for the given body part", () => {
    const stats = {
      min_sample: 5,
      parts: [
        {
          body_part: "knee",
          body_part_label: "膝",
          posts: 9,
          stages: [
            { stage: "rehab", stage_label: "リハビリ中", reached: 6, median_days: 12 },
            { stage: "race", stage_label: "大会に復帰", reached: 2, median_days: null },
          ],
        },
      ],
    } as const;
    expect(injuryTypicalStages(stats as never, "knee").map((s) => s.stage)).toEqual(["rehab"]);
    expect(injuryTypicalStages(stats as never, "ankle")).toEqual([]);
    expect(injuryTypicalStages(null, "knee")).toEqual([]);
  });

  it("formats the days to a stage in the personal history", () => {
    expect(injuryHistoryStageText({ stage_label: "ジョグ再開", days: 45 })).toBe("ジョグ再開（45日後）");
    expect(injuryHistoryStageText({ stage_label: "安静中", days: 0 })).toBe("安静中（投稿の当日）");
  });

  it("finds body parts injured more than once", () => {
    const items = [
      { body_part: "knee", body_part_label: "膝" },
      { body_part: "calf", body_part_label: "ふくらはぎ" },
      { body_part: "knee", body_part_label: "膝" },
    ];
    expect(injuryRepeatedParts(items)).toEqual([{ label: "膝", count: 2 }]);
    expect(injuryRepeatedParts([])).toEqual([]);
  });
});
