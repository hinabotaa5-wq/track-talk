// 二段階認証(認証アプリの6桁のコード)まわりの判定。画面から切り離してテストする。

// Supabase の currentLevel・nextLevel。"aal1"(パスワードなどだけ)か "aal2"(二段階認証まで済み)。
export type AssuranceLevel = string | null | undefined;

export type MfaGateState = "none" | "verify" | "setup";

// 二段階認証の確認がなくても開ける画面。ログインし直す・規約を読むなどはできるようにする。
const openPrefixes = ["/login", "/register", "/auth", "/terms", "/privacy", "/contact", "/guidelines"];

// 管理者・コーチは、二段階認証を設定するまで、これらの画面を開けない(API も拒む)。
const staffPrefixes = ["/admin", "/coach"];
// コーチの説明と応募は、まだコーチでない人のための画面なので止めない。
const staffOpenPrefixes = ["/coach/about", "/coach/apply"];

function under(pathname: string, prefixes: string[]) {
  return prefixes.some((prefix) => pathname === prefix || pathname.startsWith(`${prefix}/`));
}

// いまの画面で、二段階認証のコードを求めるか(verify)、設定を求めるか(setup)を決める。
// - 設定済みの人は、ログインのたびにコードを確かめるまでどの画面も開けない。
// - 管理者・承認済みコーチで未設定の人(mfaRequired)は、管理・コーチの画面だけ止めて設定へ案内する。
export function mfaGateState(input: {
  pathname: string;
  currentLevel: AssuranceLevel;
  nextLevel: AssuranceLevel;
  mfaRequired?: boolean;
}): MfaGateState {
  const { pathname, currentLevel, nextLevel, mfaRequired } = input;
  if (under(pathname, openPrefixes)) return "none";
  if (currentLevel === "aal1" && nextLevel === "aal2") return "verify";
  if (mfaRequired && currentLevel !== "aal2" && under(pathname, staffPrefixes) && !under(pathname, staffOpenPrefixes)) {
    return "setup";
  }
  return "none";
}

// 認証アプリのコードは6桁の数字。全角やスペースが混ざっても受け付ける。
export function normalizeTotpCode(raw: string): string {
  return raw
    .replace(/[０-９]/g, (c) => String.fromCharCode(c.charCodeAt(0) - 0xfee0))
    .replace(/\s|-/g, "");
}

export function isTotpCode(raw: string): boolean {
  return /^\d{6}$/.test(normalizeTotpCode(raw));
}

const mfaMessages: Record<string, string> = {
  mfa_verification_failed: "コードが違うか、有効期限が切れています。認証アプリの新しいコードを入れてください。",
  mfa_challenge_expired: "時間がたちすぎました。もう一度コードを入れてください。",
  mfa_factor_not_found: "二段階認証の設定が見つかりません。もう一度設定してください。",
  too_many_enrolled_mfa_factors: "設定できる数の上限です。使っていない設定を解除してください。",
  mfa_verification_rejected: "確認できませんでした。しばらくしてからもう一度試してください。",
  insufficient_aal: "先に二段階認証のコードを入れてください。",
  mfa_totp_enroll_not_enabled: "二段階認証は現在使えません。",
  mfa_totp_verify_not_enabled: "二段階認証は現在使えません。",
};

export function mfaErrorMessage(error: { code?: string; message?: string }): string {
  if (error.code && mfaMessages[error.code]) return mfaMessages[error.code];
  const message = (error.message ?? "").toLowerCase();
  if (message.includes("invalid totp") || message.includes("invalid code")) return mfaMessages.mfa_verification_failed;
  if (message.includes("aal2")) return mfaMessages.insufficient_aal;
  return "確認に失敗しました。もう一度試してください。";
}
