import { describe, expect, it } from "vitest";
import { mustCompleteProfile, mustConsentOverseas, profileIncomplete } from "./profile-gate";

describe("profileIncomplete", () => {
  it("ニックネームか種目がまだなら、決めていないとみなす", () => {
    expect(profileIncomplete(null)).toBe(false);
    expect(profileIncomplete({ nickname: null, event: "100m" })).toBe(true);
    expect(profileIncomplete({ nickname: " ", event: "100m" })).toBe(true);
    expect(profileIncomplete({ nickname: "ラン", event: null })).toBe(true);
    expect(profileIncomplete({ nickname: "ラン", event: "100m" })).toBe(false);
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
