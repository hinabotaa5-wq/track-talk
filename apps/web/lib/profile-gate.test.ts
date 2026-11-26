import { describe, expect, it } from "vitest";
import { mustCompleteProfile, mustConsentOverseas, profileGateRedirecting, profileIncomplete } from "./profile-gate";

describe("profileIncomplete", () => {
  it("ニックネームか種目がまだなら、決めていないとみなす", () => {
    expect(profileIncomplete(null)).toBe(false);
    expect(profileIncomplete({ nickname: null, event: "100m" })).toBe(true);
    expect(profileIncomplete({ nickname: " ", event: "100m" })).toBe(true);
    expect(profileIncomplete({ nickname: "ラン", event: null })).toBe(true);
    expect(profileIncomplete({ nickname: "ラン", event: "100m" })).toBe(false);
  });

  it("生まれた年の情報がない人も、決めていないとみなす（本体 #1045）。値が来ない古い API では止めない", () => {
    expect(profileIncomplete({ nickname: "ラン", event: "100m", birth_year_known: false })).toBe(true);
    expect(profileIncomplete({ nickname: "ラン", event: "100m", birth_year_known: true })).toBe(false);
    expect(profileIncomplete({ nickname: "ラン", event: "100m", birth_year_known: undefined })).toBe(false);
  });
});

describe("mustCompleteProfile", () => {
  const empty = { nickname: null, event: null };

  it("決めていない人は、決める画面と規約などのほかは開けない", () => {
    expect(mustCompleteProfile("/", empty)).toBe(true);
    expect(mustCompleteProfile("/injury/data", empty)).toBe(true);
    expect(mustCompleteProfile("/me/profile", empty)).toBe(false);
    expect(mustCompleteProfile("/terms", empty)).toBe(false);
    expect(mustCompleteProfile("/contact", empty)).toBe(false);
    expect(mustCompleteProfile("/reset-password/update", empty)).toBe(false);
  });

  it("生まれた年だけがまだの人も、プロフィール画面へ送る。退会の画面は塞がない（本体 #1045）", () => {
    const noYear = { nickname: "ラン", event: "100m", birth_year_known: false };
    expect(mustCompleteProfile("/", noYear)).toBe(true);
    expect(mustCompleteProfile("/coaching/new", noYear)).toBe(true);
    expect(mustCompleteProfile("/me/profile", noYear)).toBe(false);
    expect(mustCompleteProfile("/me/delete", noYear)).toBe(false);
    expect(mustCompleteProfile("/goodbye", noYear)).toBe(false);
    expect(mustCompleteProfile("/me/deleteX", noYear)).toBe(true);
  });

  it("決め終えた人と、まだ読み込み中の人は止めない", () => {
    expect(mustCompleteProfile("/", { nickname: "ラン", event: "100m" })).toBe(false);
    expect(mustCompleteProfile("/", null)).toBe(false);
  });
});

describe("mustConsentOverseas", () => {
  it("同意していない人は、同意の画面と規約・退会などのほかは開けない", () => {
    const missing = { overseas_consent: false };
    expect(mustConsentOverseas("/", missing)).toBe(true);
    expect(mustConsentOverseas("/me/profile", missing)).toBe(true);
    expect(mustConsentOverseas("/consent", missing)).toBe(false);
    expect(mustConsentOverseas("/privacy", missing)).toBe(false);
    expect(mustConsentOverseas("/me/delete", missing)).toBe(false);
  });

  it("同意済みの人、読み込み中の人、値が来ない古い API では止めない", () => {
    expect(mustConsentOverseas("/", { overseas_consent: true })).toBe(false);
    expect(mustConsentOverseas("/", null)).toBe(false);
    expect(mustConsentOverseas("/", {})).toBe(false);
  });

  it("同意の画面にいる間は、プロフィールの画面へ送り返さない", () => {
    expect(mustCompleteProfile("/consent", { nickname: null, event: null })).toBe(false);
  });
});

describe("profileGateRedirecting", () => {
  it("同意かニックネームを決める画面へ移す前なら true、移った後や決め終えた人は false", () => {
    expect(profileGateRedirecting("/", { nickname: null, event: null, overseas_consent: true })).toBe(true);
    expect(profileGateRedirecting("/", { nickname: "ラン", event: "100m", overseas_consent: false })).toBe(true);
    expect(profileGateRedirecting("/me/profile", { nickname: null, event: null, overseas_consent: false })).toBe(true);
    expect(profileGateRedirecting("/me/profile", { nickname: null, event: null, overseas_consent: true })).toBe(false);
    expect(profileGateRedirecting("/consent", { nickname: null, event: null, overseas_consent: false })).toBe(false);
    expect(profileGateRedirecting("/", { nickname: "ラン", event: "100m", overseas_consent: true })).toBe(false);
    expect(profileGateRedirecting("/", null)).toBe(false);
  });
});
