import { afterEach, describe, expect, it } from "vitest";
import { installEnglish, setActiveLocale } from "./i18n";
import { en } from "./i18n-en";
import { displayPriceAmount, displayYenPerUsd, formatChatDateTime, formatDateTime, formatDisplayPrice, formatYen, fromJstInput, toJstInput, todayInTokyo, yenToDisplayUsd } from "./format";

it("チャット日時は日本時間の月日・時分だけで、年と秒を省く", () => {
  expect(formatChatDateTime("2026-11-17T21:19:37Z")).toBe("11/18 6:19");
  expect(formatChatDateTime("2026-12-31T15:05:00Z")).toBe("1/1 0:05");
  expect(formatChatDateTime("2026-01-02T12:34:00+09:00")).toBe("1/2 12:34");
  expect(formatChatDateTime("invalid")).toBe("");
  expect(formatDateTime("2026-11-17T21:19:37Z")).toContain("2026");
});

describe("formatDateTime の表示言語（本体 #944）", () => {
  afterEach(() => setActiveLocale("ja"));

  it("日本語表示では従来どおり日本の書式で、日本時間で出す", () => {
    setActiveLocale("ja");
    expect(formatDateTime("2026-11-18T02:05:00Z")).toBe("2026/11/18 11:05");
  });

  it("英語表示では英語の書式で、日本時間で出す", () => {
    installEnglish(en);
    setActiveLocale("en");
    expect(formatDateTime("2026-11-18T02:05:00Z")).toBe("Nov 18, 2026, 11:05 AM");
    expect(formatDateTime("2026-11-18T14:30:00Z")).toBe("Nov 18, 2026, 11:30 PM");
  });
});

describe("todayInTokyo", () => {
  it("reads today in Japan time", () => {
    expect(todayInTokyo(new Date("2026-10-15T15:30:00Z"))).toBe("2026-10-16");
    expect(todayInTokyo(new Date("2026-10-15T14:59:00Z"))).toBe("2026-10-15");
  });
});

describe("datetime-local in Japan time", () => {
  it("writes and reads the wall clock in Japan time regardless of the device", () => {
    expect(toJstInput("2026-10-20T09:00:00.000Z")).toBe("2026-10-20T18:00");
    expect(toJstInput("2026-10-19T15:30:00.000Z")).toBe("2026-10-20T00:30");
    expect(fromJstInput("2026-10-20T18:00").toISOString()).toBe("2026-10-20T09:00:00.000Z");
    expect(fromJstInput(toJstInput("2026-12-31T14:59:00.000Z")).toISOString()).toBe("2026-12-31T14:59:00.000Z");
  });

  it("rejects values it cannot read", () => {
    expect(toJstInput("not a date")).toBe("");
    expect(Number.isNaN(fromJstInput("").getTime())).toBe(true);
    expect(Number.isNaN(fromJstInput("2026-10-20").getTime())).toBe(true);
  });
});

// 本体 #1085：選手向けの価格表示。英語表示はドルだけ（固定 ¥160=$1、セントで四捨五入・常に小数2桁）、日本語表示は従来の円のまま。
describe("formatDisplayPrice（本体 #1085）", () => {
  afterEach(() => setActiveLocale("ja"));
  const cases: [number, string][] = [[1000, "$6.25"], [500, "$3.13"], [2000, "$12.50"], [800, "$5.00"], [1500, "$9.38"]];

  it("T1: レートは固定 ¥160=$1 で、セント単位に四捨五入する", () => {
    expect(displayYenPerUsd).toBe(160);
    expect(yenToDisplayUsd(500)).toBe(3.13);
    expect(yenToDisplayUsd(1500)).toBe(9.38);
    expect(yenToDisplayUsd(0)).toBe(0);
  });

  it("T1: 英語表示（辞書読み込み済み）ではドルだけを小数2桁で出す", () => {
    installEnglish(en);
    setActiveLocale("en");
    for (const [yen, usd] of cases) {
      expect(formatDisplayPrice(yen)).toBe(usd);
      expect(displayPriceAmount(yen)).toBe(usd.slice(1));
    }
    expect(formatDisplayPrice(0)).toBe("$0.00");
    expect(formatDisplayPrice(320000)).toBe("$2,000.00");
    expect(displayPriceAmount(320000)).toBe("2,000.00");
  });

  it("T1: 日本語表示では formatYen と完全に同じ円の文字列で、数字は従来どおり", () => {
    setActiveLocale("ja");
    expect(formatDisplayPrice(1000)).toBe("1,000円");
    expect(formatDisplayPrice(500)).toBe("500円");
    for (const [yen] of cases) {
      expect(formatDisplayPrice(yen)).toBe(formatYen(yen));
      expect(displayPriceAmount(yen)).toBe(yen.toLocaleString("ja-JP"));
    }
  });

  it("英語表示でも formatYen 自体は変えない（コーチ側・精算系は円のまま）", () => {
    installEnglish(en);
    setActiveLocale("en");
    expect(formatYen(1000)).toBe("¥1,000");
  });
});
