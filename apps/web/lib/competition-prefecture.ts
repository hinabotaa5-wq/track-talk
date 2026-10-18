import { INTERNATIONAL, monthNumber, prefectureCode } from "@/lib/prefectures";

const storageKey = "track-talk.competition-prefecture";

export const competitionPrefectureCookie = "tt_competition_prefecture";

export const competitionPrefectureEvent = "track-talk:competition-prefecture";

export function readSavedCompetitionPrefecture() {
  if (typeof window === "undefined") return "";
  try {
    const raw = window.localStorage.getItem(storageKey);
    if (!raw) return "";
    return prefectureCode(raw);
  } catch {
    return "";
  }
}

export function saveCompetitionPrefecture(code: string) {
  const normalized = prefectureCode(code);
  if (!normalized || typeof window === "undefined") return;
  try {
    window.localStorage.setItem(storageKey, normalized);
  } catch {
    // 次に来たときにも Cookie から地域を戻せるよう、続ける。
  }
  if (typeof document !== "undefined") {
    document.cookie = `${competitionPrefectureCookie}=${normalized}; Path=/; Max-Age=31536000; SameSite=Lax`;
  }
  window.dispatchEvent(new Event(competitionPrefectureEvent));
}

export const regionalCompetitionsPath = "/competitions/regional";

// 地域ページに県の指定なしで来たら、保存した県へ送る。
export function regionalRedirectTarget(
  prefectureParam: string | undefined,
  saved: string | undefined,
  monthParam: string | undefined,
) {
  if (prefectureParam && regionalPrefecture(prefectureParam)) return "";
  const code = regionalPrefecture(saved);
  if (!code) return "";
  return regionalUrl(code, monthNumber(monthParam));
}

// 以前はメインタブが年間の一覧のために ?prefecture= を受け取っていた。今は国際がメインタブにあり、
// 各県は地域ページにある。
export function legacyCompetitionsTarget(prefectureParam: string | undefined, monthParam: string | undefined) {
  if (!prefectureParam) return "";
  const code = prefectureCode(prefectureParam);
  if (!code) return "";
  if (code === INTERNATIONAL) return "/competitions?view=international";
  return regionalUrl(code, monthNumber(monthParam));
}

// A prefecture for the regional page. 国際 is not one of them.
export function regionalPrefecture(raw: string | undefined) {
  const code = prefectureCode(raw);
  return code && code !== INTERNATIONAL ? code : "";
}

function regionalUrl(code: string, month: number) {
  const params = new URLSearchParams();
  if (month) params.set("month", String(month));
  params.set("prefecture", code);
  return `${regionalCompetitionsPath}?${params.toString()}`;
}
