import { describe, expect, it } from "vitest";
import { formatDateTime, formatYen } from "./format";
import { contractPeriodLabel, planDurationLabel, planIncludes, planSummary, sortTrialFirst } from "./coaching-plan";

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
    expect(planIncludes({ video_reviews: null, menu_updates: null, reply_formats: [], start_on_first_video: false })).toEqual([]);
    expect(planIncludes({ video_reviews: 4, menu_updates: "weekly", reply_formats: ["text", "video"], start_on_first_video: true })).toEqual([
      { label: "動画への返答", value: "4回まで" },
      { label: "練習メニュー", value: "毎週見直す" },
      { label: "返し方", value: "文章・解説の動画" },
      { label: "期間", value: "最初の動画を送った日から数える" },
    ]);
    expect(planIncludes({ video_reviews: 0, menu_updates: "none", reply_formats: [], start_on_first_video: false })[0].value).toBe(
      "回数を決めない（期間中いつでも）",
    );
  });
});

describe("contractPeriodLabel", () => {
  // 1か月のプランで、4/1 に始まり、終了日は待つ7日を足した 5/8。
  const waiting = {
    status: "active" as const,
    duration_days: 30,
    start_on_first_video: true,
    clock_started_at: null,
    starts_at: "2026-04-01T00:00:00Z",
    ends_at: "2026-05-08T00:00:00Z",
  };
  it("動画を待っているあいだは、いつまでに送ればよいかを出す", () => {
    const label = contractPeriodLabel(waiting, new Date("2026-04-03T00:00:00Z"));
    expect(label).toContain("最初の動画を送った日から30日");
    expect(label).toContain(formatDateTime("2026-04-08T00:00:00Z"));
  });
  it("届かずに待つ日数が過ぎたら、その時からの期間を出す", () => {
    expect(contractPeriodLabel(waiting, new Date("2026-04-10T00:00:00Z"))).not.toContain("最初の動画");
  });
  it("動画が届いたら、届いた時からの期間を出す", () => {
    const label = contractPeriodLabel(
      { ...waiting, clock_started_at: "2026-04-03T00:00:00Z", ends_at: "2026-05-03T00:00:00Z" },
      new Date("2026-04-10T00:00:00Z"),
    );
    expect(label).not.toContain("最初の動画");
    expect(label).toContain(formatDateTime("2026-04-03T00:00:00Z"));
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
