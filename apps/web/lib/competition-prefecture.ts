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
    // Keep going so the cookie can still restore the region on the next visit.
  }
  if (typeof document !== "undefined") {
    document.cookie = `${competitionPrefectureCookie}=${normalized}; Path=/; Max-Age=31536000; SameSite=Lax`;
  }
  window.dispatchEvent(new Event(competitionPrefectureEvent));
}

export const regionalCompetitionsPath = "/competitions/regional";

// Sends a bare regional visit back to the saved prefecture.
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

// The main tab used to take ?prefecture= for the yearly lists. 国際 now lives on the main tab and
// each prefecture on the regional page.
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
