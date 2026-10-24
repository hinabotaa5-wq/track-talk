import { BoundedCache } from "./bounded-cache";
import { apiFetch } from "./api";
import { fetchPostsRetrying, retainHomeFeedWindow, type PostsPage } from "./fetch-posts";
import type { HomeSort } from "./home-sort";

export type FeedSort = HomeSort;

const freshForMs = 60_000;
const prefetchSorts: FeedSort[] = ["popular", "new", "recruitment"];

type Entry = { data: PostsPage; at: number };

const cache = new BoundedCache<Entry>(48, 15 * 60_000);
const chunksCache = new BoundedCache<PostsPage[]>(4, 15 * 60_000);
export function readHomeFeedChunks(path: string) { return chunksCache.get(path); }
export function homeFeedGeneration() { return generation; }
export function saveHomeFeedChunks(path: string, chunks: PostsPage[], atGeneration: number) {
  // 大きな一覧を複数タブ分保持しない。表示中の一覧はコンポーネントが持つ。
  if (atGeneration !== generation) return;
  if (chunks.reduce((n, chunk) => n + chunk.posts.length, 0) <= 6000) chunksCache.set(path, retainHomeFeedWindow(chunks, new Set()));
  else chunksCache.delete(path);
}

const inflight = new Map<string, Promise<PostsPage>>();
// 消したあとに、消す前に始めた読み込みが書き戻さないようにする世代の番号(ログアウト後に前の人の保存一覧が残らないように)。
let generation = 0;
// 投稿の作成・編集・削除でフィードを消した時刻。これより前にサーバーが描いた画面の一覧は古い。
let homeClearedAt = 0;

export function homeFeedPath(sort: FeedSort, q: string, page: number) {
  const qs = new URLSearchParams({
    sort: sort === "recruitment" ? "new" : sort,
    page: String(page),
    per_page: "12",
  });
  if (page === 1) qs.set("pagination", "cursor");
  if (q) qs.set("q", q);
  if (sort === "recruitment") qs.set("kind", "recruitment");
  return `/v1/posts?${qs}`;
}

// pageは位置表示用。cursor指定時のAPIはOFFSETを使わない。
export function nextHomeFeedPath(sort: FeedSort, q: string, page: number, cursor?: string) {
  const path = homeFeedPath(sort, q, page);
  if (!cursor) return path;
  const [base, query] = path.split("?");
  const params = new URLSearchParams(query);
  params.set("pagination", "cursor");
  params.set("cursor", cursor);
  return `${base}?${params}`;
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

// at を渡すと、その時刻に取ったものとして入れる(古ければ、開いたあとに目立たないように取り直す)。
export function seedFeed(path: string, data: PostsPage, at = Date.now()) {
  cache.set(path, { data, at });
}

// サーバーが renderedAt に描いた一覧が、フィードを消したあとの新しいものか。
// ブラウザは戻る・進むや同じ画面への移動で前に描いた画面を使い回すので、その中の一覧で作り直したキャッシュを上書きさせない。
export function renderStamp() {
  return Date.now();
}

export function isRenderFresh(renderedAt: number) {
  return renderedAt > homeClearedAt;
}

// サーバーが描いてから少しでも時間がたった一覧(Netlify の端にキャッシュされていたもの)は、
// 別の端末で消した投稿や退会した人の投稿が残っていることがあるので、すぐ取り直す。
const renderReusedAfterMs = 10_000;

export function renderSeedTime(renderedAt: number, now = Date.now()) {
  return now - renderedAt > renderReusedAfterMs ? 0 : now;
}

export function clearHomeFeeds() {
  generation++;
  homeClearedAt = Date.now();
  chunksCache.clear();
  for (const key of [...cache.keys(), ...inflight.keys()]) {
    if (key.startsWith("/v1/posts")) {
      cache.delete(key);
      inflight.delete(key);
    }
  }
}

export function clearSavedFeeds() {
  generation++;
  for (const key of [...cache.keys(), ...inflight.keys()]) {
    if (key.startsWith("/v1/me/")) {
      cache.delete(key);
      inflight.delete(key);
    }
  }
}

function start(path: string, request: () => Promise<{ posts?: PostsPage["posts"] | null; meta: PostsPage["meta"] }>) {
  const startedAt = generation;
  const pending: Promise<PostsPage> = request()
    .then((data) => {
      const page = { posts: data.posts ?? [], meta: data.meta, request_path: path };
      if (startedAt === generation) seedFeed(path, page);
      return page;
    })
    .finally(() => {
      if (inflight.get(path) === pending) inflight.delete(path);
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
  if (!retry && isFeedFresh(path)) return Promise.resolve(readFeed(path)!);
  return run();
}

export function loadSavedFeed(path: string, token: string): Promise<PostsPage> {
  return inflight.get(path) ?? start(path, () => apiFetch<PostsPage>(path, { token }));
}

export function prefetchHomeFeeds(q: string) {
  // 検索のたびに全タブを先読みしない。従量課金・低速回線でも表示中を優先する。
  if (q) return;
  const connection = typeof navigator !== "undefined" ? (navigator as Navigator & { connection?: { saveData?: boolean; effectiveType?: string } }).connection : undefined;
  if (connection?.saveData || ["slow-2g", "2g", "3g"].includes(connection?.effectiveType ?? "")) return;
  for (const sort of prefetchSorts) {
    const path = homeFeedPath(sort, q, 1);
    if (isFeedFresh(path) || inflight.has(path)) continue;
    loadHomeFeed(path).catch(() => {});
  }
}
