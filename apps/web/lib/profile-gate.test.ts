import { describe, expect, it } from "vitest";
import { mustCompleteProfile, profileIncomplete } from "./profile-gate";

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
