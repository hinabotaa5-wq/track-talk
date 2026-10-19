import { describe, expect, it } from "vitest";
import { mainTabHref } from "./bottom-nav";

describe("mainTabHref", () => {
  it("ログインしていなくても、マイページ以外のタブはそのまま開ける", () => {
    expect(mainTabHref("home", false)).toBe("/");
    expect(mainTabHref("competitions", false)).toBe("/competitions");
    expect(mainTabHref("coaching", false)).toBe("/coaching");
    expect(mainTabHref("injury", false)).toBe("/injury");
  });

  it("マイページは、ログインしていなければログイン画面に向ける", () => {
    expect(mainTabHref("me", false)).toBe("/login");
    expect(mainTabHref("me", true)).toBe("/me");
  });
});
