import { describe, expect, it } from "vitest";
import { formatYen } from "./format";
import { planDurationLabel, planIncludes, planSummary, sortTrialFirst } from "./coaching-plan";

describe("planDurationLabel", () => {
  it("期間プランは2週間か1か月、単発は日数で出す", () => {
    expect(planDurationLabel("period", 14)).toBe("2週間");
    expect(planDurationLabel("period", 30)).toBe("1か月");
    expect(planDurationLabel("single", 7)).toBe("7日以内");
    expect(planDurationLabel("trial", 7)).toBe("7日以内");
  });
});

describe("planIncludes", () => {
  it("書いた項目だけを出す", () => {
    expect(planIncludes({ video_reviews: null, menu_updates: null, reply_formats: [] })).toEqual([]);
    expect(planIncludes({ video_reviews: 4, menu_updates: "weekly", reply_formats: ["text", "video"] })).toEqual([
      { label: "動画への返答", value: "4回まで" },
      { label: "練習メニュー", value: "毎週見直す" },
      { label: "返し方", value: "文章・解説の動画" },
    ]);
    expect(planIncludes({ video_reviews: 0, menu_updates: "none", reply_formats: [] })[0].value).toBe("回数を決めない（期間中いつでも）");
  });
});

describe("sortTrialFirst", () => {
  it("お試しを先頭にして、ほかの順番は変えない", () => {
    const plans = [
      { id: "a", kind: "single" as const },
      { id: "b", kind: "trial" as const },
      { id: "c", kind: "period" as const },
    ];
    expect(sortTrialFirst(plans).map((p) => p.id)).toEqual(["b", "a", "c"]);
  });
});

describe("planSummary", () => {
  it("種類・期間・値段を1行にする", () => {
    expect(formatYen(300000)).toBe("300,000円");
    expect(planSummary({ kind: "period", duration_days: 30, price_yen: 8000 })).toBe("期間プラン · 1か月 · 8,000円");
  });
});
