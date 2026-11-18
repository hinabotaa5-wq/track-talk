import { afterEach, expect, it } from "vitest";
import { coachDisplayText, countryFlagEmoji } from "./coach-display";
import { setActiveLocale } from "./i18n";

afterEach(() => setActiveLocale("ja"));

// 項目12〜14の共通ルール: en+訳ありだけ訳文、項目単位フォールバック。
it("enでは訳文、jaでは原文、訳なし・空では原文", () => {
  setActiveLocale("en");
  expect(coachDisplayText("テストコーチ", "Test Coach")).toBe("Test Coach");
  expect(coachDisplayText("テストコーチ", null)).toBe("テストコーチ");
  expect(coachDisplayText("テストコーチ", "")).toBe("テストコーチ");
  expect(coachDisplayText("テストコーチ", undefined)).toBe("テストコーチ");
  setActiveLocale("ja");
  expect(coachDisplayText("テストコーチ", "Test Coach")).toBe("テストコーチ");
  expect(coachDisplayText(null, "Test Coach")).toBeNull();
  setActiveLocale("en");
  expect(coachDisplayText(null, "Test Coach")).toBe("Test Coach");
});

// 本体 #932 B3: 国コードから regional indicator で国旗を作る。不正値は空文字で、例外を投げない。
it("国コードから国旗の絵文字を作る", () => {
  expect(countryFlagEmoji("JP")).toBe("🇯🇵");
  expect(countryFlagEmoji("US")).toBe("🇺🇸");
  expect(countryFlagEmoji("KE")).toBe("🇰🇪");
  for (const bad of ["", "JPN", "J", "jp", "J1", " JP", null, undefined]) {
    expect(countryFlagEmoji(bad)).toBe("");
  }
});
