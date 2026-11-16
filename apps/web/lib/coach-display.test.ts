import { afterEach, expect, it } from "vitest";
import { coachDisplayText } from "./coach-display";
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
