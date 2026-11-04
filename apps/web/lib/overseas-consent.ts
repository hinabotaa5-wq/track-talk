// 登録画面で「海外の事業者に預けること」にも同意してから登録を始めた印。
// 登録後に初めて /v1/me を読んだとき、この印があれば同意の画面を出さずにそのまま記録する。
//
// メールで登録する人は、同意した説明の版をアカウント(user_metadata)にも残す。確認メールを別のタブや
// 別の端末で開いても、もう一度同意の画面を出さずに記録できる。版は API の overseasConsentVersion とそろえる。
// API の版が先に上がっていたら記録されず、同意の画面で新しい版に同意してもらう。
export const overseasConsentVersion = "overseas-2026-10";
const metadataKey = "overseas_consent_version";

export function signupConsentMetadata() {
  return { [metadataKey]: overseasConsentVersion };
}

export function signupConsentVersion(metadata: Record<string, unknown> | null | undefined) {
  const value = metadata?.[metadataKey];
  return typeof value === "string" && value !== "" ? value : null;
}

// Google で登録する人は user_metadata を渡せないので、同じタブの保存領域に1時間だけ印を置く。
const key = "tracktalk-overseas-consent";
const maxAgeMs = 60 * 60 * 1000;

export function rememberOverseasConsent(now = Date.now()) {
  try {
    sessionStorage.setItem(key, String(now + maxAgeMs));
  } catch {
    // 保存できないときは、同意の画面で同意してもらう。
  }
}

export function takeOverseasConsent(now = Date.now()) {
  try {
    const value = sessionStorage.getItem(key);
    sessionStorage.removeItem(key);
    return value !== null && Number(value) > now;
  } catch {
    return false;
  }
}
