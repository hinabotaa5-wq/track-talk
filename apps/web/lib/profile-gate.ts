import type { Profile } from "./types";

// ニックネームと種目は、全員が決める。どちらかがまだなら true。
export function profileIncomplete(profile: Pick<Profile, "nickname" | "event"> | null) {
  return !!profile && (!profile.nickname?.trim() || !profile.event);
}

// ニックネームと種目を決める前でも開ける画面。ここ以外は、決める画面に送り返す。
const openPrefixes = ["/me/profile", "/consent", "/login", "/register", "/reset-password", "/auth", "/terms", "/privacy", "/contact", "/guidelines", "/guardian"];

// 海外の事業者に預けることへ同意する前でも開ける画面。同意しない人が退会できるよう、退会の画面も開ける。
const consentOpenPrefixes = ["/consent", "/login", "/register", "/reset-password", "/auth", "/terms", "/privacy", "/contact", "/guidelines", "/guardian", "/me/delete", "/goodbye"];

function under(pathname: string, prefixes: string[]) {
  return prefixes.some((prefix) => pathname === prefix || pathname.startsWith(`${prefix}/`));
}

export function mustCompleteProfile(pathname: string, profile: Pick<Profile, "nickname" | "event"> | null) {
  if (!profileIncomplete(profile)) return false;
  return !under(pathname, openPrefixes);
}

// いまの版の説明で同意していない人は、同意の画面に送る。古い API が値を返さないとき(undefined)は止めない。
export function mustConsentOverseas(pathname: string, profile: Pick<Profile, "overseas_consent"> | null) {
  if (profile?.overseas_consent !== false) return false;
  return !under(pathname, consentOpenPrefixes);
}
