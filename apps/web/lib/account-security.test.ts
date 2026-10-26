import { describe, expect, it } from "vitest";
import { isTotpCode, mfaErrorMessage, mfaGateState, normalizeTotpCode } from "./account-security";

describe("mfaGateState", () => {
  it("設定済みでコード未確認なら、どの画面でもコードを求める", () => {
    expect(mfaGateState({ pathname: "/", currentLevel: "aal1", nextLevel: "aal2" })).toBe("verify");
    expect(mfaGateState({ pathname: "/me/password", currentLevel: "aal1", nextLevel: "aal2" })).toBe("verify");
  });

  it("ログイン・登録・規約の画面は止めない", () => {
    for (const pathname of ["/login", "/register", "/auth/callback", "/terms", "/privacy", "/guidelines"]) {
      expect(mfaGateState({ pathname, currentLevel: "aal1", nextLevel: "aal2", mfaRequired: true })).toBe("none");
    }
  });

  it("確認済みなら止めない", () => {
    expect(mfaGateState({ pathname: "/admin", currentLevel: "aal2", nextLevel: "aal2", mfaRequired: false })).toBe("none");
  });

  it("未設定の管理者・コーチは、管理とコーチの画面だけ設定へ案内する", () => {
    const base = { currentLevel: "aal1" as const, nextLevel: "aal1" as const, mfaRequired: true };
    expect(mfaGateState({ ...base, pathname: "/admin" })).toBe("setup");
    expect(mfaGateState({ ...base, pathname: "/admin/coaching" })).toBe("setup");
    expect(mfaGateState({ ...base, pathname: "/coach" })).toBe("setup");
    expect(mfaGateState({ ...base, pathname: "/coach/requests" })).toBe("setup");
    expect(mfaGateState({ ...base, pathname: "/coach/apply" })).toBe("none");
    expect(mfaGateState({ ...base, pathname: "/coaches" })).toBe("none");
    expect(mfaGateState({ ...base, pathname: "/me/security" })).toBe("none");
    expect(mfaGateState({ ...base, pathname: "/" })).toBe("none");
  });

  it("ふつうの利用者で未設定なら止めない", () => {
    expect(mfaGateState({ pathname: "/admin", currentLevel: "aal1", nextLevel: "aal1", mfaRequired: false })).toBe("none");
    expect(mfaGateState({ pathname: "/", currentLevel: null, nextLevel: null })).toBe("none");
  });
});

describe("TOTP のコード", () => {
  it("全角・スペース・ハイフンを直してから6桁か確かめる", () => {
    expect(normalizeTotpCode("１２３ ４５６")).toBe("123456");
    expect(isTotpCode("123-456")).toBe(true);
    expect(isTotpCode("12345")).toBe(false);
    expect(isTotpCode("12345a")).toBe(false);
  });
});

describe("mfaErrorMessage", () => {
  it("コード違いは、入れ直せる文言にする", () => {
    expect(mfaErrorMessage({ code: "mfa_verification_failed" })).toContain("新しいコード");
    expect(mfaErrorMessage({ message: "Invalid TOTP code entered" })).toContain("新しいコード");
    expect(mfaErrorMessage({ message: "something" })).toBe("確認に失敗しました。もう一度試してください。");
  });
});
