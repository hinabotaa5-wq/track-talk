import { useEffect, useSyncExternalStore } from "react";
import { apiFetch } from "@/lib/api";

// メモタブの「〇〇大会まであと何日」に使う、目標にセットした大会。1人1大会で、別の大会をセットすると置き換わる。
export type TargetMeet = {
  source: "major" | "competition";
  ref: string;
  name: string;
  starts_on: string;
  ends_on: string | null;
};

type Response = { target_meet: TargetMeet | null };

// 大会一覧の各カードとメモタブで同じ値を見るので、ログイン中の人の分を1つだけ持つ。
// undefined は読み込み前。
let current: { token: string; target: TargetMeet | null | undefined } | null = null;
let loading: string | null = null;
const listeners = new Set<() => void>();

function emit(next: typeof current) {
  current = next;
  listeners.forEach((listener) => listener());
}

function subscribe(listener: () => void) {
  listeners.add(listener);
  return () => listeners.delete(listener);
}

function load(token: string) {
  if (loading === token || current?.token === token) return;
  loading = token;
  apiFetch<Response>("/v1/me/target-meet", { token })
    .then((d) => { if (loading === token) emit({ token, target: d.target_meet }); })
    // 読めなかったときは「未セット」と見せず何も出さない。次に開いたときに読み直す。
    .catch(() => {})
    .finally(() => { if (loading === token) loading = null; });
}

export async function setTargetMeet(token: string, source: TargetMeet["source"], ref: string) {
  const d = await apiFetch<Response>("/v1/me/target-meet", { method: "PUT", token, body: JSON.stringify({ source, ref }) });
  emit({ token, target: d.target_meet });
}

export async function clearTargetMeet(token: string) {
  await apiFetch<Response>("/v1/me/target-meet", { method: "DELETE", token });
  emit({ token, target: null });
}

// token が null(未ログイン)のときは、セットされていない扱い。
export function useTargetMeet(token: string | null): { target: TargetMeet | null; loaded: boolean } {
  useEffect(() => {
    if (token) load(token);
  }, [token]);
  const snapshot = useSyncExternalStore(subscribe, () => current, () => null);
  if (!token) return { target: null, loaded: true };
  if (snapshot?.token !== token || snapshot.target === undefined) return { target: null, loaded: false };
  return { target: snapshot.target, loaded: true };
}

// 東京の日付どうしの差(日)。
function daysBetween(from: string, to: string) {
  const [fy, fm, fd] = from.split("-").map(Number);
  const [ty, tm, td] = to.split("-").map(Number);
  return Math.round((Date.UTC(ty, tm - 1, td) - Date.UTC(fy, fm - 1, fd)) / 86_400_000);
}

export type Countdown = { kind: "before"; days: number } | { kind: "today" } | { kind: "ongoing" } | { kind: "over" };

// 初日の前は「あと何日」、初日は「今日」、2日目以降の会期中は「開催中」、最終日を過ぎたら「終了」。
export function targetMeetCountdown(target: Pick<TargetMeet, "starts_on" | "ends_on">, today: string): Countdown {
  const days = daysBetween(today, target.starts_on);
  if (days > 0) return { kind: "before", days };
  if (days === 0) return { kind: "today" };
  return daysBetween(today, target.ends_on ?? target.starts_on) >= 0 ? { kind: "ongoing" } : { kind: "over" };
}
