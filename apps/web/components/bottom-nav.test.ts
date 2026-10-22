import { describe, expect, it } from "vitest";
import { activeMainTab, hasBackOnlyHeader, hidesHeader, mainTabs, mainTabHref, showsBackButton, showsHeaderAccountActions } from "./bottom-nav";

describe("mainTabHref", () => {
  it("下のメニューは ホーム→大会→メモ→コーチ→ケガ の順", () => {
    expect(mainTabs.map((tab) => tab.id)).toEqual(["home", "competitions", "memo", "coaching", "injury"]);
  });

  it("ログインしていなくても、マイページ以外のタブはそのまま開ける", () => {
    expect(mainTabHref("home", false)).toBe("/");
    expect(mainTabHref("memo", false)).toBe("/memo");
    expect(mainTabHref("competitions", false)).toBe("/competitions");
    expect(mainTabHref("coaching", false)).toBe("/coaching");
    expect(mainTabHref("injury", false)).toBe("/injury");
  });

  it("マイページは、ログインしていなければログイン画面に向ける", () => {
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

describe("hidesHeader", () => {
  it("はじめのニックネーム・種目の画面と、新しいパスワードの画面ではヘッダーを出さない", () => {
    expect(hidesHeader("/me/profile", "?onboarding=1")).toBe(true);
    expect(hidesHeader("/reset-password/update")).toBe(true);
    expect(hidesHeader("/reset-password/update", "?code=x")).toBe(true);
    expect(hidesHeader("/me/profile", "", true)).toBe(true);
  });

  it("マイページから開いたニックネームの変更や、ほかの画面では出す", () => {
    expect(hidesHeader("/me/profile")).toBe(false);
    expect(hidesHeader("/reset-password")).toBe(false);
    expect(hidesHeader("/")).toBe(false);
  });
});


describe("showsHeaderAccountActions", () => {
  it("通知・マイページ・管理アイコンを通常ページで表示し、戻る画面や投稿画面では隠す", () => {
    expect(showsHeaderAccountActions("/")).toBe(true);
    expect(showsHeaderAccountActions("/me")).toBe(true);
    expect(showsHeaderAccountActions("/me/push")).toBe(true);
    expect(showsHeaderAccountActions("/upload")).toBe(false);
    expect(showsHeaderAccountActions("/posts/new")).toBe(false);
    expect(showsHeaderAccountActions("/menu-contest")).toBe(false);
  });
});
