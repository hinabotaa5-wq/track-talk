import { describe, expect, it } from "vitest";
import { activeMainTab, hasBackOnlyHeader, mainTabs, mainTabHref, showsBackButton } from "./bottom-nav";

describe("mainTabHref", () => {
  it("下のメニューは ホーム→大会→メモ→コーチ→ケガ の順", () => {
    expect(mainTabs.map((tab) => tab.id)).toEqual(["home", "competitions", "memo", "coaching", "injury"]);
  });

  it("ログインしていなくても、メモとマイページ以外のタブはそのまま開ける", () => {
    expect(mainTabHref("home", false)).toBe("/");
    expect(mainTabHref("competitions", false)).toBe("/competitions");
    expect(mainTabHref("coaching", false)).toBe("/coaching");
    expect(mainTabHref("injury", false)).toBe("/injury");
  });

  it("メモとマイページは、ログインしていなければログイン画面に向ける", () => {
    expect(mainTabHref("memo", false)).toBe("/login");
    expect(mainTabHref("memo", true)).toBe("/memo");
    expect(activeMainTab("/memo")).toBe("memo");
    expect(mainTabHref("me", false)).toBe("/login");
    expect(mainTabHref("me", true)).toBe("/me");
  });
});

describe("hasBackOnlyHeader", () => {
  it("マイページから開いたログイン画面以外のログイン系の画面は、戻るボタンだけのヘッダーにする", () => {
    expect(hasBackOnlyHeader("/login", "")).toBe(false);
    expect(hasBackOnlyHeader("/login", "?next=/upload")).toBe(true);
    expect(hasBackOnlyHeader("/register")).toBe(true);
    expect(hasBackOnlyHeader("/reset-password")).toBe(true);
    expect(hasBackOnlyHeader("/reset-password/update")).toBe(true);
  });

  it("マイページの中の画面は、どれも戻るボタンだけのヘッダーにする。マイページ本体は対象外", () => {
    expect(hasBackOnlyHeader("/me")).toBe(false);
    for (const path of ["/me/profile", "/me/badge", "/me/push", "/me/sense", "/me/coaching/requests"]) {
      expect(hasBackOnlyHeader(path)).toBe(true);
      expect(showsBackButton(path)).toBe(true);
    }
  });

  it("ケガのグループの画面は戻るボタンだけ。ケガタブの一覧とデータは対象外", () => {
    expect(hasBackOnlyHeader("/injury/hamstring-strain")).toBe(true);
    expect(hasBackOnlyHeader("/injury")).toBe(false);
    expect(hasBackOnlyHeader("/injury/data")).toBe(false);
    expect(showsBackButton("/injury/hamstring-strain")).toBe(true);
    expect(showsBackButton("/injury/hamstring-strain/stage")).toBe(true);
    expect(showsBackButton("/injury/data")).toBe(false);
  });
});
