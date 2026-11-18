import { afterEach, expect, it } from "vitest";
import { postDisplayText } from "./post-display";
import { setActiveLocale } from "./i18n";
afterEach(() => setActiveLocale("ja"));
it("uses English only for nonempty translations and falls back per field", () => {
  setActiveLocale("en");
  expect(postDisplayText("原文", "English")).toBe("English");
  for (const value of [null, undefined, ""]) expect(postDisplayText("原文", value)).toBe("原文");
  expect(postDisplayText(undefined, undefined)).toBeNull();
  setActiveLocale("ja");
  expect(postDisplayText("原文", "English")).toBe("原文");
});
