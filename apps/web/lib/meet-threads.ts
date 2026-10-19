import { apiFetch } from "./api";
import type { Competition, DomesticMajor, MeetThread } from "./types";

// 今日開催中の大会の実況スレッド。失敗しても画面は出せるので、空配列に丸める。
export async function fetchTodayMeetThreads(token?: string | null): Promise<MeetThread[]> {
  try {
    const data = await apiFetch<{ meets: MeetThread[] }>("/v1/meet-threads/today", { token });
    return data.meets ?? [];
  } catch {
    return [];
  }
}

// 大会タブの一覧の大会に、いま実況スレッドがあれば返す。開催中かどうかは API(ホームと同じ判定)に任せ、ここでは突き合わせるだけ。
export function meetThreadForMajor(meets: MeetThread[], major: DomesticMajor) {
  return meets.find((meet) => meet.major_slug === major.slug && meet.major_year === major.year);
}

export function meetThreadForCompetition(meets: MeetThread[], competition: Competition) {
  return meets.find((meet) => meet.competition_id === competition.id);
}
