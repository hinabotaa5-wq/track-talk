import type { Profile } from "./types";

// ニックネームと種目は、全員が決める。どちらかがまだなら true。
export function profileIncomplete(profile: Pick<Profile, "nickname" | "event"> | null) {
  return !!profile && (!profile.nickname?.trim() || !profile.event);
}

// ニックネームと種目を決める前でも開ける画面。ここ以外は、決める画面に送り返す。
const openPrefixes = ["/me/profile", "/login", "/register", "/reset-password", "/auth", "/terms", "/privacy", "/contact", "/guidelines", "/guardian"];

export function mustCompleteProfile(pathname: string, profile: Pick<Profile, "nickname" | "event"> | null) {
  if (!profileIncomplete(profile)) return false;
  return !openPrefixes.some((prefix) => pathname === prefix || pathname.startsWith(`${prefix}/`));
}
