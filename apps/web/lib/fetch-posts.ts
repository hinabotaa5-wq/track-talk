import { apiFetch } from "./api";
import type { Meta, Post } from "./types";

export type PostsPage = { posts: Post[]; meta: Meta; request_path?: string; unloaded?: boolean; unavailable_ids?: string[] };

type FetchPosts = (path: string, signal: AbortSignal) => Promise<PostsPage>;

export type FetchPostsRetryOptions = {
  budgetMs?: number;
  pauseMs?: number;
  attemptTimeoutMs?: number;
  signal?: AbortSignal;
  now?: () => number;
  sleep?: (ms: number) => Promise<void>;
  fetchImpl?: FetchPosts;
};

export const homeFeedServerTimeoutMs = 4_000;

const retryBudgetMs = 60_000;
const retryPauseMs = 3_000;
const retryAttemptTimeoutMs = 20_000;

const defaultSleep = (ms: number) => new Promise<void>((resolve) => setTimeout(resolve, ms));

function attemptSignal(ms: number, parent?: AbortSignal) {
  const controller = new AbortController();
  const timer = setTimeout(() => controller.abort(), ms);
  const onParentAbort = () => controller.abort();
  if (parent) {
    if (parent.aborted) controller.abort();
    else parent.addEventListener("abort", onParentAbort, { once: true });
  }
  return {
    signal: controller.signal,
    cancel() {
      clearTimeout(timer);
      parent?.removeEventListener("abort", onParentAbort);
    },
  };
}

export async function fetchPostsRetrying(
  path: string,
  options: FetchPostsRetryOptions = {},
): Promise<PostsPage> {
  const budgetMs = options.budgetMs ?? retryBudgetMs;
  const pauseMs = options.pauseMs ?? retryPauseMs;
  const attemptTimeoutMs = options.attemptTimeoutMs ?? retryAttemptTimeoutMs;
  const now = options.now ?? Date.now;
  const sleep = options.sleep ?? defaultSleep;
  const fetchImpl =
    options.fetchImpl ?? ((requestPath, signal) => apiFetch<PostsPage>(requestPath, { signal }));
  const deadline = now() + budgetMs;
  let lastError: unknown;

  while (!options.signal?.aborted) {
    const remaining = deadline - now();
    if (remaining <= 0) break;
    const attempt = attemptSignal(Math.min(attemptTimeoutMs, remaining), options.signal);
    try {
      const data = await fetchImpl(path, attempt.signal);
      return { posts: data.posts ?? [], meta: data.meta };
    } catch (error) {
      lastError = error;
      if (options.signal?.aborted) break;
    } finally {
      attempt.cancel();
    }
    const after = deadline - now();
    if (after <= 0 || options.signal?.aborted) break;
    await sleep(Math.min(pauseMs, after));
  }

  if (lastError instanceof Error) throw lastError;
  throw new Error("投稿の取得に失敗しました。");
}

// 画面から遠いページはID・形だけ残し、本文などのデータを解放する。
// 座標を保ったまま、再表示されたページだけ同じ取得条件で読み直す。
export function retainHomeFeedWindow(chunks: PostsPage[], visibleIDs: Set<string>, maxPosts = 600): PostsPage[] {
  if (chunks.reduce((n, chunk) => n + (chunk.unloaded ? 0 : chunk.posts.length), 0) <= maxPosts) return chunks;
  const center = Math.max(0, chunks.findIndex((chunk) => chunk.posts.some((post) => visibleIDs.has(post.id))));
  const retained = new Set<number>();
  let count = 0;
  const isVisible = chunks.map((chunk) => chunk.posts.some((post) => visibleIDs.has(post.id)));
  const order = chunks.map((_, i) => i).sort((a, b) => Number(isVisible[b]) - Number(isVisible[a]) || Math.abs(a - center) - Math.abs(b - center));
  for (const index of order) {
    const chunk = chunks[index];
    if (!chunk.unloaded && (count + chunk.posts.length <= maxPosts || chunk.posts.some((post) => visibleIDs.has(post.id)))) {
      retained.add(index); count += chunk.posts.length;
    }
  }
  return chunks.map((chunk, i) => chunk.unloaded || retained.has(i) ? chunk : {
    ...chunk, unloaded: true,
    posts: chunk.posts.map((post) => ({
      id: post.id, title: "", kind: post.kind, thumbnail_url: post.thumbnail_url,
      youtube_shorts: post.youtube_shorts, instagram_type: post.instagram_type,
      mine: false, like_count: 0, comment_count: 0, liked_by_me: false, saved_by_me: false,
      created_at: post.created_at, updated_at: post.updated_at,
    })),
  });
}
