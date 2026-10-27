// @vitest-environment jsdom
import { beforeEach, describe, expect, it } from "vitest";
import { rememberOverseasConsent, takeOverseasConsent } from "./overseas-consent";

describe("overseas-consent", () => {
  beforeEach(() => sessionStorage.clear());

  it("登録画面で残した印は、1回だけ取り出せる", () => {
    rememberOverseasConsent(1000);
    expect(takeOverseasConsent(2000)).toBe(true);
    expect(takeOverseasConsent(2000)).toBe(false);
  });

  it("1時間を過ぎた印と、無い印は使わない", () => {
    expect(takeOverseasConsent(0)).toBe(false);
    rememberOverseasConsent(0);
    expect(takeOverseasConsent(60 * 60 * 1000 + 1)).toBe(false);
  });
});
