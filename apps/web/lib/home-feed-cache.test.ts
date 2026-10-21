import { describe, expect, it, vi } from "vitest";

let resolve: (v: unknown) => void = () => {};
vi.mock("./api", () => ({
  apiFetch: () => new Promise((r) => (resolve = r)),
}));

import { clearHomeFeeds, clearSavedFeeds, isRenderFresh, loadSavedFeed, readFeed, renderSeedTime, savedFeedPath } from "./home-feed-cache";

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
