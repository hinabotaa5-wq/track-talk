import { apiFetch } from "./api";
import { fetchPostsRetrying, type PostsPage } from "./fetch-posts";
import type { HomeSort } from "./home-sort";

export type FeedSort = Exclude<HomeSort, "saved">;

const freshForMs = 60_000;
const prefetchSorts: FeedSort[] = ["popular", "new", "recruitment"];

type Entry = { data: PostsPage; at: number };

const cache = new Map<string, Entry>();
const inflight = new Map<string, Promise<PostsPage>>();

export function homeFeedPath(sort: FeedSort, q: string, page: number) {
  const qs = new URLSearchParams({
    sort: sort === "recruitment" ? "new" : sort,
    page: String(page),
    per_page: "20",
  });
  if (q) qs.set("q", q);
  if (sort === "recruitment") qs.set("kind", "recruitment");
  return `/v1/posts?${qs}`;
}

export function savedFeedPath(q: string, page: number) {
  const qs = new URLSearchParams({ page: String(page), per_page: "20" });
  if (q) qs.set("q", q);
  return `/v1/me/saves?${qs}`;
}

export function readFeed(path: string) {
  return cache.get(path)?.data;
}

export function isFeedFresh(path: string) {
  const hit = cache.get(path);
  return !!hit && Date.now() - hit.at < freshForMs;
}

export function seedFeed(path: string, data: PostsPage) {
  cache.set(path, { data, at: Date.now() });
}

export function clearSavedFeeds() {
  for (const key of [...cache.keys(), ...inflight.keys()]) {
    if (key.startsWith("/v1/me/")) {
      cache.delete(key);
      inflight.delete(key);
    }
  }
}

function start(path: string, request: () => Promise<{ posts?: PostsPage["posts"] | null; meta: PostsPage["meta"] }>) {
  const pending = request()
    .then((data) => {
      const page = { posts: data.posts ?? [], meta: data.meta };
      seedFeed(path, page);
      return page;
    })
    .finally(() => {
      inflight.delete(path);
    });
  inflight.set(path, pending);
  return pending;
}

// retry: keep retrying while the API wakes up. Prefetches use a single attempt.
export function loadHomeFeed(path: string, retry = false): Promise<PostsPage> {
  const run = () =>
    start(path, () => (retry ? fetchPostsRetrying(path) : apiFetch<PostsPage>(path)));
  const pending = inflight.get(path);
  if (pending) return retry ? pending.catch(run) : pending;
  return run();
}

export function loadSavedFeed(path: string, token: string): Promise<PostsPage> {
  return inflight.get(path) ?? start(path, () => apiFetch<PostsPage>(path, { token }));
}

export function prefetchHomeFeeds(q: string) {
  for (const sort of prefetchSorts) {
    const path = homeFeedPath(sort, q, 1);
    if (isFeedFresh(path) || inflight.has(path)) continue;
    loadHomeFeed(path).catch(() => {});
  }
}

export function prefetchSavedFeed(q: string, token: string) {
  const path = savedFeedPath(q, 1);
  if (isFeedFresh(path) || inflight.has(path)) return;
  loadSavedFeed(path, token).catch(() => {});
}
