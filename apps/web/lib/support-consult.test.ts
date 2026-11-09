import { expect, it } from "vitest";
import { answerRows, emptyAnswers, priceHintLabel } from "./support-consult";

it("価格の目安は下限と上限がそろったときだけ出す", () => {
  expect(priceHintLabel(5000, 12000)).toBe("5,000円〜12,000円");
  expect(priceHintLabel(null, null)).toBe("");
  expect(priceHintLabel(5000, null)).toBe("");
});

it("コーチが読む回答は、空の項目を出さず区分ごとにまとまる", () => {
  const rows = answerRows({
    ...emptyAnswers(), goal_decided: true, competition_name: "秋季記録会", competition_date: "2026-12-20", event: "100m",
    goals: ["pb", "final"], personal_best: "12.50", injury_now: false, main_worry: "スタートで出遅れる", start_timing: "asap",
  }, "100m");
  expect(rows.find((r) => r.label === "大会")?.value).toBe("秋季記録会（2026-12-20）");
  expect(rows.find((r) => r.label === "大会での目標")?.value).toBe("自己ベストを出す・決勝・上位大会に進む");
  expect(rows.find((r) => r.label === "今の痛み・怪我")?.value).toBe("いいえ");
  expect(rows.find((r) => r.label === "目標記録")).toBeUndefined();
  expect(rows.find((r) => r.label === "コーチに聞きたいこと")).toBeUndefined();
  expect(rows.find((r) => r.label === "その他、コーチに伝えたいこと")).toBeUndefined();
});

it("目標の大会が未定なら、時期の目安を出す", () => {
  const rows = answerRows({ ...emptyAnswers(), goal_decided: false, timing_hint: "3_6m" }, "100m");
  expect(rows.find((r) => r.label === "大会")?.value).toBe("未定（3〜6か月先）");
});
