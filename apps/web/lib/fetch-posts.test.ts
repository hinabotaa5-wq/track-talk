import { describe, expect, it } from "vitest";
import { fetchPostsRetrying, type PostsPage } from "./fetch-posts";

const page: PostsPage = {
  posts: [],
  meta: { page: 1, per_page: 20, total: 0 },
};

describe("fetchPostsRetrying", () => {
  it("returns the first successful response", async () => {
    let calls = 0;
    let clock = 0;
    const result = await fetchPostsRetrying("/v1/posts", {
      budgetMs: 10_000,
      pauseMs: 3_000,
      now: () => clock,
      sleep: async (ms) => {
        clock += ms;
      },
      fetchImpl: async () => {
        calls += 1;
        if (calls < 3) throw new Error("down");
        return page;
      },
    });
    expect(result).toEqual(page);
    expect(calls).toBe(3);
    expect(clock).toBe(6_000);
  });

  it("stops when the budget is spent", async () => {
    let calls = 0;
    let clock = 0;
    await expect(
      fetchPostsRetrying("/v1/posts", {
        budgetMs: 7_000,
        pauseMs: 3_000,
        now: () => clock,
        sleep: async (ms) => {
          clock += ms;
        },
        fetchImpl: async () => {
          calls += 1;
          throw new Error("down");
        },
      }),
    ).rejects.toThrow("down");
    expect(calls).toBe(3);
    expect(clock).toBe(7_000);
  });

  it("does not call the API after the request is cancelled", async () => {
    const controller = new AbortController();
    controller.abort();
    let calls = 0;
    await expect(
      fetchPostsRetrying("/v1/posts", {
        signal: controller.signal,
        fetchImpl: async () => {
          calls += 1;
          return page;
        },
      }),
    ).rejects.toThrow("投稿の取得に失敗しました。");
    expect(calls).toBe(0);
  });
});
