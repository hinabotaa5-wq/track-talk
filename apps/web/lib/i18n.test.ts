// @vitest-environment jsdom
import { afterEach, describe, expect, it } from "vitest";
import {
  localePromptAnswered,
  mainTabLabel,
  markLocalePromptAnswered,
  parseLocale,
  readStoredLocale,
  storeLocale,
  t,
} from "./i18n";

afterEach(() => {
  localStorage.clear();
});

describe("parseLocale", () => {
  it("en だけ en、ほかは ja", () => {
    expect(parseLocale("en")).toBe("en");
    expect(parseLocale("ja")).toBe("ja");
    expect(parseLocale("")).toBe("ja");
    expect(parseLocale("EN")).toBe("ja");
    expect(parseLocale(undefined)).toBe("ja");
    expect(parseLocale(null)).toBe("ja");
  });
});

describe("t", () => {
  it("ja は日本語を返す", () => {
    expect(t("ja", "nav.home")).toBe("ホーム");
    expect(t("ja", "coach.offers.accept")).toBe("引き受ける");
  });

  it("en は英語を返す", () => {
    expect(t("en", "nav.home")).toBe("Home");
    expect(t("en", "coach.offers.accept")).toBe("Accept");
  });

  it("{name} を置き換える", () => {
    expect(t("ja", "coach.newOffers", { count: 2, minutes: 30 })).toBe(
      "新しい依頼が2件あります — あと30分で終了",
    );
    expect(t("en", "coach.newOffers", { count: 2, minutes: 30 })).toBe(
      "2 new request(s) — closing in 30 min",
    );
  });

  it("足りない変数は {name} のまま残す", () => {
    expect(t("en", "coach.newOffers", { count: 1 })).toContain("{minutes}");
  });
});

describe("stored locale", () => {
  it("保存と読み出しができる", () => {
    storeLocale("en");
    expect(readStoredLocale()).toBe("en");
    storeLocale("ja");
    expect(readStoredLocale()).toBe("ja");
  });
});

describe("mainTabLabel", () => {
  it("タブ名を言語で切り替える", () => {
    expect(mainTabLabel("home", "ja")).toBe("ホーム");
    expect(mainTabLabel("home", "en")).toBe("Home");
    expect(mainTabLabel("me", "en")).toBe("My Page");
  });
});

describe("ログイン画面の文言", () => {
  it("日英どちらも空白・キー名にならない", () => {
    expect(t("ja", "login.title")).toBe("ログイン");
    expect(t("en", "login.title")).toBe("Log in");
    expect(t("en", "login.submit")).toBe("Log in");
    expect(t("en", "login.forgot")).toBe("Forgot password");
  });
});

describe("言語プロンプトの記録", () => {
  it("答えるまでは未回答、答えたら記録される", () => {
    expect(localePromptAnswered()).toBe(false);
    markLocalePromptAnswered();
    expect(localePromptAnswered()).toBe(true);
  });
});
