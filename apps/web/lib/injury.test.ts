import { describe, expect, it } from "vitest";
import { canPostInStage, injuryCelebrationKey, injuryFaceHue, injuryFaceInitial, injuryFaceOverflow, shouldCelebrate, emptyInjuryJoinAnswers, injuryGroupPath, injuryJoinPayload, injuryJoinStepError, localDateString, sortInjuryGroups, injuryHistoryStageText, injuryRepeatedParts, injuryStageStatText, injuryTypicalStages, isInjuryBodyPart, isInjuryStage } from "./injury";

describe("injury", () => {
  it("validates body part codes", () => {
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

describe("injury group join", () => {
  const today = "2026-10-15";
  const filled = { injuredOn: "2026-09-20", place: "練習中", situation: "着地でひねった", stage: "rehab" } as const;

  it("asks for the date, rejecting a missing or future one", () => {
    expect(injuryJoinStepError("injuredOn", emptyInjuryJoinAnswers, today)).not.toBe("");
    expect(injuryJoinStepError("injuredOn", { ...filled, injuredOn: "2026-10-16" }, today)).toBe("未来の日付は選べません");
    expect(injuryJoinStepError("injuredOn", { ...filled, injuredOn: today }, today)).toBe("");
  });

  it("asks for the place and the situation within their limits", () => {
    expect(injuryJoinStepError("place", { ...filled, place: "  " }, today)).not.toBe("");
    expect(injuryJoinStepError("place", { ...filled, place: "あ".repeat(101) }, today)).not.toBe("");
    expect(injuryJoinStepError("place", filled, today)).toBe("");
    expect(injuryJoinStepError("situation", { ...filled, situation: "" }, today)).not.toBe("");
    expect(injuryJoinStepError("situation", { ...filled, situation: "あ".repeat(501) }, today)).not.toBe("");
    expect(injuryJoinStepError("situation", filled, today)).toBe("");
  });

  it("asks for a known stage", () => {
    expect(injuryJoinStepError("stage", { ...filled, stage: "" }, today)).not.toBe("");
    expect(injuryJoinStepError("stage", filled, today)).toBe("");
  });

  it("sends trimmed answers in the API's field names", () => {
    expect(injuryJoinPayload({ ...filled, place: " 練習中 ", situation: " ひねった " })).toEqual({
      injured_on: "2026-09-20",
      place: "練習中",
      situation: "ひねった",
      stage: "rehab",
    });
  });

  it("formats the local date and the group path", () => {
    expect(localDateString(new Date(2026, 0, 5))).toBe("2026-01-05");
    expect(injuryGroupPath("lower-back")).toBe("/injury/lower-back");
  });

  it("lists joined groups first, then the biggest", () => {
    const me = { stage: "rehab", stage_label: "リハビリ中", injured_on: "2026-09-20", place: "a", situation: "b" } as const;
    const sorted = sortInjuryGroups([
      { body_part: "ankle", body_part_label: "足首", members: 5 },
      { body_part: "knee", body_part_label: "膝", members: 1, me },
      { body_part: "foot", body_part_label: "足裏・足部", members: 5 },
      { body_part: "calf", body_part_label: "ふくらはぎ", members: 9 },
    ]);
    expect(sorted.map((item) => item.body_part)).toEqual(["knee", "calf", "foot", "ankle"]);
  });
});

describe("injury group roadmap", () => {
  it("lets a member post only in their own stage", () => {
    expect(canPostInStage("jogging", "jogging")).toBe(true);
    expect(canPostInStage("rehab", "jogging")).toBe(false);
  });

  it("celebrates only the first arrival at the goal", () => {
    expect(shouldCelebrate("race", false)).toBe(true);
    expect(shouldCelebrate("race", true)).toBe(false);
    expect(shouldCelebrate("practice", false)).toBe(false);
    expect(injuryCelebrationKey("knee")).toBe("injury-celebrated:knee");
  });

  it("gives the same name the same face", () => {
    expect(injuryFaceHue("はると")).toBe(injuryFaceHue("はると"));
    expect(injuryFaceHue("はると")).toBeGreaterThanOrEqual(0);
    expect(injuryFaceHue("はると")).toBeLessThan(360);
    expect(injuryFaceInitial(" ハルト")).toBe("ハ");
    expect(injuryFaceInitial("")).toBe("?");
  });

  it("counts the members the faces leave out", () => {
    expect(injuryFaceOverflow(23, 5)).toBe(18);
    expect(injuryFaceOverflow(3, 5)).toBe(0);
  });
});
