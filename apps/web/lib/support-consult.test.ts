import { describe, expect, it } from "vitest";
import { answerRows, emptyAnswers, priceHintLabel, supportProrationExample, supportTermsLines } from "./support-consult";

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

describe("1ヶ月プランの途中終了の表示", () => {
  it("算定例は API と同じ日割り（円未満切り捨て・終える日は経過扱い）", () => {
    expect(supportProrationExample(9000, 30)).toEqual({ total: 30, elapsed: 10, remaining: 20, refund: 6000 });
    expect(supportProrationExample(10000, 3)).toEqual({ total: 3, elapsed: 1, remaining: 2, refund: 6666 });
    expect(supportProrationExample(5000, 1)).toEqual({ total: 1, elapsed: 1, remaining: 0, refund: 0 });
  });
  it("算定例・返金の時期と方法・チャットの閲覧期間を出す（中止延期の扱いは出ない）", () => {
    const text = supportTermsLines(9000, 30).map((l) => `${l.label} ${l.text}`).join("\n");
    expect(text).toContain("残り20日分の6,000円を返金");
    expect(text).toContain("解約料・違約金はかかりません");
    expect(text).toContain("お支払いに使ったカードへ返金");
    expect(text).toContain("実際に始まった日から1ヶ月（両端を含む30日）");
    expect(text).not.toContain("中止");
    expect(text).toContain("契約が終わった時点（期間の満了・途中での終了とも）で、選手・コーチ・保護者のいずれからも見られなくなります");
    expect(text).toContain("終わるまでのあいだに、やりとり（文章とメニュー）をダウンロードできます");
    expect(text).not.toContain("90日");
    expect(text).toContain("全額を返金します");
  });
});
