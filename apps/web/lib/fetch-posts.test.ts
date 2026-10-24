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

it("遠いページの本文を解放してもIDと続きの取得条件は保つ", async () => {
  const { retainHomeFeedWindow } = await import("./fetch-posts");
  const pages = Array.from({ length: 60 }, (_, i) => ({ posts: Array.from({ length: 12 }, (_, j) => ({ id: `${i}-${j}`, body: "長い本文".repeat(200), title: "タイトル", kind: "text" } as import("./types").Post)), meta: { page: i+1, per_page: 12, total: 720 }, request_path: `request-${i}` }));
  const retained = retainHomeFeedWindow(pages, new Set(["59-0"]));
  expect(retained.filter((page) => !page.unloaded).flatMap((page) => page.posts)).toHaveLength(600);
  expect(retained[0].unloaded).toBe(true);
  expect(retained[0].posts[0].body).toBeUndefined();
  expect(retained[0].posts[0].id).toBe("0-0");
  expect(retained[0].request_path).toBe("request-0");
  expect(retained[59].posts[0].body).toBe(pages[59].posts[0].body);
});

it("離れた場所のフォーカスと表示中のページを優先して保持する", async () => {
  const { retainHomeFeedWindow } = await import("./fetch-posts");
  const pages = Array.from({ length: 60 }, (_, i) => ({ posts: Array.from({ length: 12 }, (_, j) => ({ id: `${i}-${j}`, title: "投稿", kind: "text" } as import("./types").Post)), meta: { page: i+1, per_page: 12, total: 720 } }));
  const retained = retainHomeFeedWindow(pages, new Set(["0-0", "59-0"]));
  expect(retained[0].unloaded).toBeUndefined();
  expect(retained[59].unloaded).toBeUndefined();
  expect(retained.filter((page) => !page.unloaded).flatMap((page) => page.posts)).toHaveLength(600);
});
