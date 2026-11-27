import { afterEach, expect, it } from "vitest";
import { englishLoaded, setActiveLocale } from "./i18n";
import { displayPriceAmount, formatDisplayPrice, formatYen } from "./format";

// 本体 #1085 T8：英語辞書を読み込む前は、表示言語が en でも円表示にフォールバックする（表示が混ざらないように）。
// 辞書を読み込むテストと同じモジュールを共有しないよう、別ファイルにしている。
afterEach(() => setActiveLocale("ja"));

it("T8: 英語辞書が未読み込みなら、locale が en でも円で出す", () => {
  setActiveLocale("en");
  expect(englishLoaded()).toBe(false);
  expect(formatDisplayPrice(1000)).toBe("1,000円");
  expect(formatDisplayPrice(1000)).toBe(formatYen(1000));
  expect(displayPriceAmount(1000)).toBe("1,000");
});
