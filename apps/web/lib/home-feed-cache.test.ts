import { describe, expect, it, vi } from "vitest";

let resolve: (v: unknown) => void = () => {};
vi.mock("./api", () => ({
  apiFetch: vi.fn(() => new Promise((r) => (resolve = r))),
}));

import { clearHomeFeeds, clearSavedFeeds, isRenderFresh, loadSavedFeed, readFeed, renderSeedTime, savedFeedPath, readHomeFeedChunks, saveHomeFeedChunks, homeFeedGeneration } from "./home-feed-cache";

describe("saved feed cache", () => {
  it("does not write back a request that started before clearing", async () => {
    const path = savedFeedPath("", 1);
    const pending = loadSavedFeed(path, "old-token");
    clearSavedFeeds();
    resolve({ posts: [{ id: "p1" }], meta: { page: 1, per_page: 20, total: 1 } });
    await pending;
    expect(readFeed(path)).toBeUndefined();
  });
});

describe("server-rendered home feed", () => {
  it("is stale once the feeds were cleared after it was rendered", () => {
    const renderedAt = Date.now() - 5_000;
    expect(isRenderFresh(renderedAt)).toBe(true);
    clearHomeFeeds();
    expect(isRenderFresh(renderedAt)).toBe(false);
    expect(isRenderFresh(Date.now() + 1_000)).toBe(true);
  });
});

describe("home feed rendered a while ago", () => {
  it("is fetched again when it came from the edge cache", () => {
    const now = 1_000_000;
    expect(renderSeedTime(now - 2_000, now)).toBe(now);
    expect(renderSeedTime(now - 60_000, now)).toBe(0);
  });
});

it("詳細から戻るための追加ページを保持し、無効化前の一覧は復活させない", () => {
  const path = "/v1/posts?sort=new&page=1";
  const chunks = [{ posts: [], meta: { page: 2, per_page: 12, total: 24 } }];
  const atGeneration = homeFeedGeneration();
  saveHomeFeedChunks(path, chunks, atGeneration);
  expect(readHomeFeedChunks(path)).toEqual(chunks);
  clearHomeFeeds();
  saveHomeFeedChunks(path, chunks, atGeneration);
  expect(readHomeFeedChunks(path)).toBeUndefined();
});

it("別の条件のキャッシュは上限を超えたら古いものから削除する", async () => {
  const { seedFeed } = await import("./home-feed-cache");
  clearHomeFeeds();
  const data = { posts: [], meta: { page: 1, per_page: 12, total: 0 } };
  for (let i = 0; i < 60; i++) seedFeed(`/v1/posts?q=${i}`, data);
  expect(readFeed("/v1/posts?q=0")).toBeUndefined();
  expect(readFeed("/v1/posts?q=59")).toEqual(data);
});

it("カーソルの続きでも検索条件と募集タブを保つ", async () => {
  const { homeFeedPath, nextHomeFeedPath } = await import("./home-feed-cache");
  const first = new URL(homeFeedPath("recruitment", "東京", 1), "https://test.invalid");
  expect(first.searchParams.get("pagination")).toBe("cursor");
  const next = new URL(nextHomeFeedPath("recruitment", "東京", 2, "abc+/="), "https://test.invalid");
  expect(next.searchParams.get("cursor")).toBe("abc+/=");
  expect(next.searchParams.get("q")).toBe("東京");
  expect(next.searchParams.get("kind")).toBe("recruitment");
  expect(next.searchParams.get("pagination")).toBe("cursor");
});


it("新鮮な一覧を再利用し、検索と低速回線では別タブを先読みしない", async () => {
  const { apiFetch } = await import("./api");
  const { seedFeed, homeFeedPath, loadHomeFeed, prefetchHomeFeeds } = await import("./home-feed-cache");
  clearHomeFeeds();
  vi.mocked(apiFetch).mockClear();
  const data = { posts: [], meta: { page: 1, per_page: 12, total: 0 } };
  const path = homeFeedPath("new", "", 1);
  seedFeed(path, data);
  expect(await loadHomeFeed(path)).toEqual(data);
  prefetchHomeFeeds("検索語");
  vi.stubGlobal("navigator", { connection: { saveData: true } });
  try {
    prefetchHomeFeeds("");
    expect(apiFetch).not.toHaveBeenCalled();
  } finally { vi.unstubAllGlobals(); }
});

it("古い世代の保存処理で新しい一覧を消さない", () => {
  const path = "/v1/posts?generation-test";
  const oldGeneration = homeFeedGeneration();
  clearHomeFeeds();
  const chunks = [{ posts: [], meta: { page: 1, per_page: 12, total: 0 } }];
  saveHomeFeedChunks(path, chunks, homeFeedGeneration());
  saveHomeFeedChunks(path, chunks, oldGeneration);
  expect(readHomeFeedChunks(path)).toEqual(chunks);
});


it("認証初期化で保存一覧を消してもホームの取得と復元を無効化しない", async () => {
  const { loadHomeFeed } = await import("./home-feed-cache");
  clearHomeFeeds();
  const path = "/v1/posts?saved-clear-race";
  const pending = loadHomeFeed(path);
  const homeGeneration = homeFeedGeneration();
  clearSavedFeeds();
  const data = { posts: [], meta: { page: 1, per_page: 12, total: 0 } };
  resolve(data);
  await pending;
  expect(readFeed(path)).toMatchObject(data);
  saveHomeFeedChunks(path, [data], homeGeneration);
  expect(readHomeFeedChunks(path)).toEqual([data]);
});
