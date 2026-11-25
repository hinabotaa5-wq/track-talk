import type { Profile } from "./types";

type GateProfile = Pick<Profile, "nickname" | "event"> & Partial<Pick<Profile, "birth_year_known">>;

// ニックネームと種目は、全員が決める。どちらかがまだなら true。
// 生まれた年（本体 #1045）も全員が登録する。年の情報（birth_year か旧方式の birth_month）がない人も true。
// 古い API が birth_year_known を返さないとき（undefined）は、年では止めない。
export function profileIncomplete(profile: GateProfile | null) {
  return !!profile && (!profile.nickname?.trim() || !profile.event || profile.birth_year_known === false);
}

// 生まれた年をまだ登録していない人なら true（プロフィール画面に年の欄を出す）。
export function birthYearMissing(profile: Partial<Pick<Profile, "birth_year_known">> | null) {
  return profile?.birth_year_known === false;
}

// ニックネーム・種目・生まれた年を決める前でも開ける画面。ここ以外は、決める画面に送り返す。
// 年を登録したくない人が退会できるよう、退会の画面と退会後の画面も開ける（本体 #1045）。
const openPrefixes = ["/me/profile", "/consent", "/login", "/register", "/reset-password", "/auth", "/terms", "/privacy", "/contact", "/guidelines", "/guardian", "/me/delete", "/goodbye"];

// 海外の事業者に預けることへ同意する前でも開ける画面。同意しない人が退会できるよう、退会の画面も開ける。
const consentOpenPrefixes = ["/consent", "/login", "/register", "/reset-password", "/auth", "/terms", "/privacy", "/contact", "/guidelines", "/guardian", "/me/delete", "/goodbye"];

function under(pathname: string, prefixes: string[]) {
  return prefixes.some((prefix) => pathname === prefix || pathname.startsWith(`${prefix}/`));
}

export function mustCompleteProfile(pathname: string, profile: GateProfile | null) {
  if (!profileIncomplete(profile)) return false;
  return !under(pathname, openPrefixes);
}

// いまの版の説明で同意していない人は、同意の画面に送る。古い API が値を返さないとき(undefined)は止めない。
export function mustConsentOverseas(pathname: string, profile: Pick<Profile, "overseas_consent"> | null) {
  if (profile?.overseas_consent !== false) return false;
  return !under(pathname, consentOpenPrefixes);
}

// いまの画面を、同意かニックネームを決める画面へ移す前提の状態なら true。
// 移すまでのあいだ起動画面を閉じずに残し、ホームなどが一瞬見えないようにする。
export function profileGateRedirecting(
  pathname: string,
  profile: (GateProfile & Pick<Profile, "overseas_consent">) | null,
) {
  return mustConsentOverseas(pathname, profile) || mustCompleteProfile(pathname, profile);
}
