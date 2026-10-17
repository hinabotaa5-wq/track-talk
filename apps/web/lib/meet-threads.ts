import { apiFetch } from "./api";
import type { MeetThread } from "./types";

// 今日開催中の大会の実況スレッド。失敗しても画面は出せるので、空配列に丸める。
export async function fetchTodayMeetThreads(token?: string | null): Promise<MeetThread[]> {
  try {
    const data = await apiFetch<{ meets: MeetThread[] }>("/v1/meet-threads/today", { token });
    return data.meets ?? [];
  } catch {
    return [];
  }
}
