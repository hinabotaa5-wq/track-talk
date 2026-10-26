import { describe, expect, it } from "vitest";
import { isRecentRecoverySession, recoveryWindowSeconds } from "./password-recovery";

function token(claims: object) {
  const body = btoa(JSON.stringify(claims)).replace(/\+/g, "-").replace(/\//g, "_").replace(/=+$/, "");
  return `header.${body}.signature`;
}

const now = 1_800_000_000;

describe("isRecentRecoverySession", () => {
  it("再設定リンクでのログインなら開く", () => {
    expect(isRecentRecoverySession(token({ amr: [{ method: "recovery", timestamp: now - 60 }] }), now)).toBe(true);
  });

  it("ふつうのログインでは開かない", () => {
    expect(isRecentRecoverySession(token({ amr: [{ method: "password", timestamp: now - 60 }] }), now)).toBe(false);
    expect(isRecentRecoverySession(token({ amr: [{ method: "oauth", timestamp: now - 60 }] }), now)).toBe(false);
  });

  it("再設定リンクでのログインから時間が経ったら開かない", () => {
    expect(isRecentRecoverySession(token({ amr: [{ method: "recovery", timestamp: now - recoveryWindowSeconds - 1 }] }), now)).toBe(false);
  });

  it("壊れたトークンや amr のないトークンでは開かない", () => {
    expect(isRecentRecoverySession(undefined, now)).toBe(false);
    expect(isRecentRecoverySession("not-a-jwt", now)).toBe(false);
    expect(isRecentRecoverySession("a.!!!.c", now)).toBe(false);
    expect(isRecentRecoverySession(token({}), now)).toBe(false);
  });
});
