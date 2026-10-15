import { apiFetch } from "@/lib/api";
import type { Competition } from "@/lib/types";

const freshForMs = 5 * 60 * 1000;
const listError = "大会一覧を取得できませんでした。";

type CompetitionList = {
  items: Competition[];
  error: string;
  at: number;
};

const cache = new Map<string, CompetitionList>();
const inflight = new Map<string, Promise<CompetitionList>>();

// month 0 means the whole season.
export function competitionCacheKey(prefecture: string, month: number) {
  return `${prefecture}:${month || "season"}`;
}

export function readCompetitions(prefecture: string, month: number) {
  return cache.get(competitionCacheKey(prefecture, month));
}

export function clearCompetitions() {
  cache.clear();
  inflight.clear();
}

export function loadCompetitions(prefecture: string, month: number, token: string) {
  const key = competitionCacheKey(prefecture, month);
  const hit = cache.get(key);
  if (hit && Date.now() - hit.at < freshForMs) return Promise.resolve(hit);

  const pending = inflight.get(key);
  if (pending) return pending;

  const query = month ? `prefecture=${prefecture}&month=${month}` : `prefecture=${prefecture}`;
  const request = apiFetch<{ competitions: Competition[] }>(`/v1/competitions?${query}`, { token })
    .then((data) => {
      const entry = { items: data.competitions ?? [], error: "", at: Date.now() };
      cache.set(key, entry);
      return entry;
    })
    .catch(() => {
      const previous = cache.get(key);
      if (previous && !previous.error) return previous;
      return { items: [] as Competition[], error: listError, at: 0 };
    })
    .finally(() => {
      inflight.delete(key);
    });

  inflight.set(key, request);
  return request;
}
