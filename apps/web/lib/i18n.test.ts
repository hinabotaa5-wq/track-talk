// @vitest-environment jsdom
import { afterEach, describe, expect, it } from "vitest";
import {
  detectBrowserLocale,
  mainTabLabel,
  parseLocale,
  t,
} from "./i18n";

const realLanguage = Object.getOwnPropertyDescriptor(window.navigator, "language");
const realLanguages = Object.getOwnPropertyDescriptor(window.navigator, "languages");

afterEach(() => {
  localStorage.clear();
  if (realLanguage) Object.defineProperty(window.navigator, "language", realLanguage);
  if (realLanguages) Object.defineProperty(window.navigator, "languages", realLanguages);
});

function setBrowserLanguage(language: unknown, languages?: unknown) {
  Object.defineProperty(window.navigator, "language", { value: language, configurable: true });
  if (languages !== undefined) {
    Object.defineProperty(window.navigator, "languages", { value: languages, configurable: true });
  }
}

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
    expect(t("ja", "nav.home")).toBe("フィード");
    expect(t("ja", "coach.offers.accept")).toBe("引き受ける");
  });

  it("en は英語を返す", () => {
    expect(t("en", "nav.home")).toBe("Feed");
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

describe("detectBrowserLocale", () => {
  it("en で始まる値（en, en-US, en-GB 等）だけ英語", () => {
    for (const value of ["en", "en-US", "en-GB", "EN-us"]) {
      setBrowserLanguage(value, []);
      expect(detectBrowserLocale()).toBe("en");
    }
  });

  it("ja・その他の言語・取れないときは日本語", () => {
    for (const value of ["ja", "ja-JP", "fr-FR", "de", "", null, undefined]) {
      setBrowserLanguage(value, []);
      expect(detectBrowserLocale()).toBe("ja");
    }
  });

  it("language が取れないときは languages[0] を見る", () => {
    setBrowserLanguage("", ["en-US"]);
    expect(detectBrowserLocale()).toBe("en");
    setBrowserLanguage(undefined, ["fr-FR"]);
    expect(detectBrowserLocale()).toBe("ja");
    setBrowserLanguage(undefined, []);
    expect(detectBrowserLocale()).toBe("ja");
  });
});

describe("mainTabLabel", () => {
  it("タブ名を言語で切り替える", () => {
    expect(mainTabLabel("home", "ja")).toBe("フィード");
    expect(mainTabLabel("home", "en")).toBe("Feed");
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


