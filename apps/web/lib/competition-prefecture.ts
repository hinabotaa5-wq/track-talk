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

export function competitionsRedirectTarget(
  prefectureParam: string | undefined,
  saved: string | undefined,
  monthParam: string | undefined,
) {
  if (prefectureParam && prefectureCode(prefectureParam)) return "";
  if (!saved) return "";
  const code = prefectureCode(saved);
  if (!code || code === INTERNATIONAL) return "";
  const params = new URLSearchParams();
  const month = monthNumber(monthParam);
  if (month) params.set("month", String(month));
  params.set("prefecture", code);
  return `/competitions?${params.toString()}`;
}

export function competitionsPath(prefecture: string) {
  if (!prefecture) return "/competitions";
  const code = prefectureCode(prefecture);
  if (!code) return "/competitions";
  return `/competitions?prefecture=${code}`;
}
