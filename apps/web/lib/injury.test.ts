import { describe, expect, it } from "vitest";
import { canPostInStage, injuryCelebrationKey, injuryFaceHue, injuryFaceInitial, injuryFaceOverflow, shouldCelebrate, isCurrentInjuryStage, emptyInjuryJoinAnswers, injuryGroupPath, injuryJoinPayload, injuryJoinStepError, injuryJoinStepsFor, localMonthString, recentMonths, togglePreChange,
  toggleHindsight, sortInjuryGroups, injuryHistoryStageText, injuryRepeatedParts, injuryStageStatText, injuryTypicalStages, isInjuryBodyPart, isInjuryStage, emptyInjuryStageChangeAnswers, injuryElapsedText, injuryStageChangeError, injuryStageChangePayload, toggleInjuryAction, injuryGroupDescriptions, injuryGroupTitle, injuryPercent, injuryRecoveryRangeText, injuryBodyParts } from "./injury";

describe("injury", () => {
  it("validates body part codes", () => {
    expect(isInjuryBodyPart("shin-splints")).toBe(true);
    expect(isInjuryBodyPart("lower-back")).toBe(false);
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
  const thisMonth = "2026-10";
  const filled = {
    diagnosis: "",
    severity: "moderate",
    sawDoctor: false,
    injuredMonth: "2026-09",
    place: "tartan",
    activity: "sprint",
    preChanges: ["more-volume"],
    hindsight: ["rest-early"],
    stage: "rehab",
  } as const;
  const answers = { ...filled, preChanges: [...filled.preChanges], hindsight: [...filled.hindsight] };

  it("asks for the injury's name only in the other group", () => {
    expect(injuryJoinStepsFor("sprain")[0]).toBe("severity");
    expect(injuryJoinStepsFor("other")[0]).toBe("diagnosis");
    expect(injuryJoinStepsFor("other").at(-1)).toBe("stage");
    expect(injuryJoinStepError("diagnosis", { ...answers, diagnosis: "  " }, thisMonth)).not.toBe("");
    expect(injuryJoinStepError("diagnosis", { ...answers, diagnosis: "あ".repeat(41) }, thisMonth)).not.toBe("");
    expect(injuryJoinStepError("diagnosis", { ...answers, diagnosis: "わからない" }, thisMonth)).toBe("");
  });

  it("asks for the month, rejecting a missing or future one", () => {
    expect(injuryJoinStepError("injuredMonth", emptyInjuryJoinAnswers, thisMonth)).not.toBe("");
    expect(injuryJoinStepError("injuredMonth", { ...answers, injuredMonth: "2026-11" }, thisMonth)).toBe("これから先の月は選べません");
    expect(injuryJoinStepError("injuredMonth", { ...answers, injuredMonth: thisMonth }, thisMonth)).toBe("");
  });

  it("needs an answer for every choice", () => {
    for (const step of ["severity", "sawDoctor", "place", "activity", "preChanges", "hindsight", "stage"] as const) {
      expect(injuryJoinStepError(step, emptyInjuryJoinAnswers, thisMonth)).not.toBe("");
      expect(injuryJoinStepError(step, answers, thisMonth)).toBe("");
    }
  });

  it("keeps nothing-changed apart from the other changes", () => {
    expect(togglePreChange(["fatigue"], "none")).toEqual(["none"]);
    expect(togglePreChange(["none"], "fatigue")).toEqual(["fatigue"]);
    expect(togglePreChange(["fatigue", "new-shoes"], "fatigue")).toEqual(["new-shoes"]);
  });

  it("sends the answers in the API's field names", () => {
    expect(injuryJoinPayload("sprain", { ...answers, diagnosis: "無視される" })).toEqual({
      severity: "moderate",
      saw_doctor: false,
      injured_month: "2026-09",
      place: "tartan",
      activity: "sprint",
      pre_changes: ["more-volume"],
      hindsight: ["rest-early"],
      stage: "rehab",
    });
    expect(injuryJoinPayload("other", { ...answers, diagnosis: " 疲労骨折 " })).toMatchObject({ diagnosis: "疲労骨折" });
  });

  it("lists the recent months, newest first", () => {
    const months = recentMonths(new Date("2026-02-10T03:00:00Z"), 3);
    expect(months).toEqual([
      { value: "2026-02", label: "2026年2月" },
      { value: "2026-01", label: "2026年1月" },
      { value: "2025-12", label: "2025年12月" },
    ]);
    expect(localMonthString(new Date("2026-01-05T03:00:00Z"))).toBe("2026-01");
    // 日本時間の月初の朝は、UTC ではまだ前の月。「今月」は日本時間で決める。
    expect(localMonthString(new Date("2026-10-31T20:00:00Z"))).toBe("2026-11");
    expect(recentMonths(new Date("2026-10-31T20:00:00Z"), 2)[0].value).toBe("2026-11");
    expect(injuryGroupPath("shin-splints")).toBe("/injury/shin-splints");
  });

  it("lists groups in the fixed order, with other last", () => {
    const me = { stage: "rehab", stage_label: "リハビリ中", injured_month: "2026-09", pre_changes: [] } as const;
    const sorted = sortInjuryGroups([
      { body_part: "other", body_part_label: "その他", members: 9 },
      { body_part: "elbow", body_part_label: "肘の痛み", members: 0 },
      { body_part: "sprain", body_part_label: "捻挫", members: 1, me: { ...me, pre_changes: [] } },
      { body_part: "hamstring-strain", body_part_label: "ハムストリングの肉離れ", members: 5 },
    ]);
    expect(sorted.map((item) => item.body_part)).toEqual(["hamstring-strain", "sprain", "elbow", "other"]);
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

  it("treats every stage before the race as a current injury", () => {
    expect(isCurrentInjuryStage("practice")).toBe(true);
    expect(isCurrentInjuryStage("resting")).toBe(true);
    expect(isCurrentInjuryStage("race")).toBe(false);
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

describe("injury stage change", () => {
  const answers = { ...emptyInjuryStageChangeAnswers, stage: "jogging" as const, actions: ["stretch" as const] };

  it("needs a new stage and at least one thing done", () => {
    expect(injuryStageChangeError(emptyInjuryStageChangeAnswers, "rehab")).toBe("新しい段階を選んでください");
    expect(injuryStageChangeError({ ...answers, stage: "rehab" }, "rehab")).toContain("いまと同じ段階");
    expect(injuryStageChangeError({ ...answers, actions: [] }, "rehab")).toContain("1つ以上");
    expect(injuryStageChangeError({ ...answers, note: "あ".repeat(301) }, "rehab")).toContain("300文字");
    expect(injuryStageChangeError({ ...answers, note: "あ".repeat(300) }, "rehab")).toBe("");
  });

  it("keeps 何もしなかった apart from the other choices", () => {
    expect(toggleInjuryAction(["stretch", "bath"], "nothing")).toEqual(["nothing"]);
    expect(toggleInjuryAction(["nothing"], "icing")).toEqual(["icing"]);
    expect(toggleInjuryAction(["icing", "bath"], "icing")).toEqual(["bath"]);
  });

  it("publishes a note only when there is one", () => {
    expect(injuryStageChangePayload({ ...answers, note: "  焦らない  ", notePublic: true })).toEqual({
      stage: "jogging",
      actions: ["stretch"],
      note: "焦らない",
      note_public: true,
    });
    expect(injuryStageChangePayload({ ...answers, note: "  ", notePublic: true }).note_public).toBe(false);
  });

  it("says roughly how long ago the injury was, by month", () => {
    const now = new Date(2026, 9, 15);
    expect(injuryElapsedText("2026-10", now)).toBe("今月ケガ");
    expect(injuryElapsedText("2026-08", now)).toBe("ケガから約2か月");
    expect(injuryElapsedText("2025-10", now)).toBe("ケガから約1年");
    expect(injuryElapsedText("2024-07", now)).toBe("ケガから約2年3か月");
    expect(injuryElapsedText("", now)).toBe("");
  });
});

describe("injury public page", () => {
  it("describes every group and names その他 naturally", () => {
    for (const part of injuryBodyParts) expect(injuryGroupDescriptions[part.value]).toBeTruthy();
    expect(injuryGroupTitle("sprain")).toBe("捻挫の仲間");
    expect(injuryGroupTitle("other")).toBe("その他のケガの仲間");
  });

  it("shows the recovery range in weeks, or in months when it is long", () => {
    expect(injuryRecoveryRangeText(14, 35)).toBe("約2〜5週間");
    expect(injuryRecoveryRangeText(21, 21)).toBe("約3週間");
    expect(injuryRecoveryRangeText(0, 3)).toBe("約1週間");
    expect(injuryRecoveryRangeText(45, 120)).toBe("約2〜4か月");
  });

  it("rounds shares to whole percents", () => {
    expect(injuryPercent(1, 3)).toBe(33);
    expect(injuryPercent(0, 0)).toBe(0);
  });
});

describe("injury hindsight", () => {
  it("keeps 'don't know' apart from the other choices", () => {
    expect(toggleHindsight(["warmup"], "none")).toEqual(["none"]);
    expect(toggleHindsight(["none"], "gear")).toEqual(["gear"]);
  });
});
