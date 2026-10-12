import { apiFetch } from "./api";
import type { Meta, Post } from "./types";

export type PostsPage = { posts: Post[]; meta: Meta };

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
