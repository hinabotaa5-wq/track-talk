import { afterEach, describe, expect, it } from "vitest";
import { coachOfferItemsError, offerTitles } from "./coach-offer";
import { premiumConfirmRows } from "./premium-confirm";
import { reviewConfirmRows } from "./purchase-confirm";
import { installEnglish, setActiveLocale } from "./i18n";
import { en } from "./i18n-en";

// 添削項目・見本動画（本体 #1086）。W4・W9：購入確認の「得られるもの」、A2 相当の画面側の入力チェック。
afterEach(() => setActiveLocale("ja"));

const standardInput = {
  method: "card" as const, priceYen: 1000, hasVideo: true, question: "相談", eventLabel: "100m",
  videoFileName: "run.mp4", videoFileSizeBytes: 1, coachName: "テストコーチ",
};

describe("得られるもの（購入確認）", () => {
  it("W4: スタンダードの確認に、コーチ詳細と同じ並びのタイトルを連結した行が出る", () => {
    const rows = reviewConfirmRows({ ...standardInput, offerTitles: offerTitles(["スタート局面の詳しい分析"]) });
    const row = rows.find((r) => r.label === "得られるもの");
    expect(row?.value).toBe("良い点、改善点、おすすめのドリル、コーチなら明日から何をするか、スタート局面の詳しい分析");
    // 内容の直後に置く。
    expect(rows.map((r) => r.label).slice(0, 2)).toEqual(["内容", "得られるもの"]);
  });

  it("W4: 追加項目のないコーチは固定4項目だけ", () => {
    const rows = reviewConfirmRows({ ...standardInput, offerTitles: offerTitles([]) });
    expect(rows.find((r) => r.label === "得られるもの")?.value).toBe("良い点、改善点、おすすめのドリル、コーチなら明日から何をするか");
  });

  it("W9: プレミアムの購入確認には「得られるもの」の行を出さない", () => {
    const rows = premiumConfirmRows({ priceYen: 30000, eventLabel: "100m", coachName: "テストコーチ", pb: "", goalTime: "", groups: [{ title: "レース全体", count: 1 }] });
    expect(rows.some((r) => r.label === "得られるもの")).toBe(false);
  });

  it("W6: 英語表示では行名と固定項目が英語、追加項目は原文のまま", () => {
    installEnglish(en);
    setActiveLocale("en");
    const rows = reviewConfirmRows({ ...standardInput, offerTitles: offerTitles(["スタート局面の詳しい分析"]) });
    expect(rows.find((r) => r.label === "What you get")?.value).toBe("Strong point, What to improve, Recommended drill, What I would do starting tomorrow if I were you, スタート局面の詳しい分析");
  });
});

describe("追加項目の入力チェック（API と同じ規則）", () => {
  it("上限ちょうど（5件・30字）は通り、6件・31字・空・重複・固定項目・旧例文は弾く", () => {
    const thirty = "あ".repeat(30);
    expect(coachOfferItemsError([thirty, "b", "c", "d", "e"])).toBe("");
    expect(coachOfferItemsError(["a", "b", "c", "d", "e", "f"])).toBe("追加項目は5件までです");
    expect(coachOfferItemsError([thirty + "い"])).toBe("追加項目のタイトルは30文字以内で入力してください");
    expect(coachOfferItemsError([" "])).toBe("追加項目のタイトルを入力してください");
    expect(coachOfferItemsError(["接地", " 接地"])).toBe("同じタイトルの追加項目が重なっています");
    for (const fixed of ["良い点", "改善点", "おすすめのドリル", "コーチなら明日から何をするか", "コーチだったら明日から何をするか"]) {
      expect(coachOfferItemsError([fixed])).toContain("追加項目にはできません");
    }
  });
});
