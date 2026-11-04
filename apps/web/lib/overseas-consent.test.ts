// @vitest-environment jsdom
import { beforeEach, describe, expect, it } from "vitest";
import {
  overseasConsentVersion,
  rememberOverseasConsent,
  signupConsentMetadata,
  signupConsentVersion,
  takeOverseasConsent,
} from "./overseas-consent";

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

describe("signupConsentVersion", () => {
  it("登録画面で残した版を、アカウントの情報から読み出す", () => {
    expect(signupConsentVersion(signupConsentMetadata())).toBe(overseasConsentVersion);
    expect(signupConsentVersion({ overseas_consent_version: "overseas-2000-01" })).toBe("overseas-2000-01");
  });

  it("版が無い・形が違うときは使わない", () => {
    expect(signupConsentVersion(undefined)).toBeNull();
    expect(signupConsentVersion({})).toBeNull();
    expect(signupConsentVersion({ overseas_consent_version: "" })).toBeNull();
    expect(signupConsentVersion({ overseas_consent_version: true })).toBeNull();
  });
});
