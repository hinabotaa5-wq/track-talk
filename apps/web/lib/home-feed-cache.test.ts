import { describe, expect, it, vi } from "vitest";

let resolve: (v: unknown) => void = () => {};
vi.mock("./api", () => ({
  apiFetch: () => new Promise((r) => (resolve = r)),
}));

import { clearSavedFeeds, loadSavedFeed, readFeed, savedFeedPath } from "./home-feed-cache";

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
