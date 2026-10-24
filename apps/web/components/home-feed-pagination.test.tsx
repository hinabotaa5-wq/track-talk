/** @vitest-environment jsdom */
import { act, type ReactNode } from "react";
import { createRoot, type Root } from "react-dom/client";
import { afterEach, beforeEach, expect, it, vi } from "vitest";
import { HomeFeed } from "./home-feed";
import { loadHomeFeed, readHomeFeedChunks } from "@/lib/home-feed-cache";
import type { PostsPage } from "@/lib/fetch-posts";
import type { Post } from "@/lib/types";

vi.mock("@/lib/home-feed-cache", () => ({
  homeFeedPath: (sort: string, q: string, page: number) => `/v1/posts?sort=${sort}&q=${q}&page=${page}&per_page=12`,
  nextHomeFeedPath: (sort: string, q: string, page: number, cursor: string) => `/v1/posts?sort=${sort}&q=${q}&page=${page}&pagination=cursor&cursor=${cursor}`,
  isFeedFresh: () => true,
  loadHomeFeed: vi.fn(),
  prefetchHomeFeeds: vi.fn(),
  readHomeFeedChunks: vi.fn(),
  saveHomeFeedChunks: vi.fn(),
  homeFeedGeneration: () => 0,
}));
vi.mock("./feed-card", () => ({ FeedCard: ({ post }: { post: Post }) => <a href={`/posts/${post.id}`}>{post.title}</a> }));
vi.mock("./masonry-feed", () => ({ MasonryFeed: ({ children, onEndReached }: { children: ReactNode; onEndReached?: () => void }) => <div><button data-end onClick={() => onEndReached?.()}>末尾付近へ</button>{children}</div> }));
const request = vi.mocked(loadHomeFeed);
const post = (id: string) => ({ id, title: id, kind: "text" }) as Post;
const chunk = (page: number, ids: string[], total = 36): PostsPage => ({ posts: ids.map(post), meta: { page, per_page: 12, total } });
let container: HTMLDivElement;
let root: Root;
beforeEach(() => {
  (globalThis as typeof globalThis & { IS_REACT_ACT_ENVIRONMENT: boolean }).IS_REACT_ACT_ENVIRONMENT = true;
  request.mockReset();
  container = document.createElement("div"); document.body.appendChild(container); root = createRoot(container);
});
afterEach(async () => { await act(async () => root.unmount()); container.remove(); });
const ids = () => [...container.querySelectorAll("a")].map((a) => a.textContent);
const end = () => (container.querySelector("[data-end]") as HTMLButtonElement).click();

it("末尾に近づいてから次のページだけを読み、重複と多重リクエストを防ぐ", async () => {
  let finish!: (data: PostsPage) => void;
  request.mockImplementation(() => new Promise((resolve) => { finish = resolve; }));
  await act(async () => root.render(<HomeFeed sort="new" q="陸上" page={1} initial={chunk(1, ["a", "b"])} />));
  expect(request).not.toHaveBeenCalled();
  await act(async () => { end(); end(); });
  expect(request).toHaveBeenCalledTimes(1);
  expect(request).toHaveBeenCalledWith(expect.stringContaining("q=陸上&page=2&per_page=12"));
  expect(ids()).toEqual(["a", "b"]);
  await act(async () => finish(chunk(2, ["b", "c"])));
  expect(ids()).toEqual(["a", "b", "c"]);
});

it("追加の取得失敗でも既存の投稿を残し、再試行で続けられる", async () => {
  request.mockRejectedValueOnce(new Error("offline"));
  await act(async () => root.render(<HomeFeed sort="popular" q="" page={1} initial={chunk(1, ["a"])} />));
  await act(async () => end());
  expect(ids()).toEqual(["a"]);
  expect(container.textContent).toContain("再試行");
  await act(async () => end());
  expect(request).toHaveBeenCalledTimes(1);
  request.mockResolvedValueOnce(chunk(2, ["b"], 24));
  const retry = [...container.querySelectorAll("button")].find((b) => b.textContent?.includes("再試行"))!;
  await act(async () => retry.click());
  expect(ids()).toEqual(["a", "b"]);
  await act(async () => end());
  expect(request).toHaveBeenCalledTimes(2);
});

it("並び替え後に前の追加取得が届いても、新しい一覧へ混ぜない", async () => {
  let finish!: (data: PostsPage) => void;
  request.mockImplementation(() => new Promise((resolve) => { finish = resolve; }));
  await act(async () => root.render(<HomeFeed key="new" sort="new" q="" page={1} initial={chunk(1, ["new"])} />));
  await act(async () => end());
  await act(async () => root.render(<HomeFeed key="popular" sort="popular" q="" page={1} initial={chunk(1, ["popular"])} />));
  await act(async () => finish(chunk(2, ["old-response"])));
  expect(ids()).toEqual(["popular"]);
});

it("詳細から戻ったときは追加ページを復元し、その続きから読み込む", async () => {
  vi.mocked(readHomeFeedChunks).mockReturnValueOnce([chunk(1, ["a"]), chunk(2, ["b"])]);
  request.mockResolvedValueOnce(chunk(3, ["c"]));
  await act(async () => root.render(<HomeFeed sort="new" q="" page={1} initial={chunk(1, ["a"])} />));
  expect(ids()).toEqual(["a", "b"]);
  await act(async () => end());
  expect(request).toHaveBeenCalledWith(expect.stringContaining("page=3"));
  expect(ids()).toEqual(["a", "b", "c"]);
});


it("総件数がなくてもカーソルを使って続きを読み、終端で止まる", async () => {
  const first = { ...chunk(1, ["a"], 0), meta: { page: 1, per_page: 12, total: 0, has_more: true, next_cursor: "next-token" } };
  request.mockResolvedValueOnce({ ...chunk(2, ["b"], 0), meta: { page: 2, per_page: 12, total: 0, has_more: false, next_cursor: "" } });
  await act(async () => root.render(<HomeFeed sort="new" q="陸上" page={1} initial={first} />));
  await act(async () => end());
  expect(request).toHaveBeenCalledWith(expect.stringContaining("pagination=cursor&cursor=next-token"));
  expect(ids()).toEqual(["a", "b"]);
  await act(async () => end());
  expect(request).toHaveBeenCalledTimes(1);
});

it("古いページの未取得状態より、後のページの実データを優先する", async () => {
  vi.mocked(readHomeFeedChunks).mockReturnValueOnce([
    { ...chunk(1, ["a"]), unloaded: true, unavailable_ids: ["a"] },
    chunk(2, ["a", "b"]),
  ]);
  await act(async () => root.render(<HomeFeed sort="new" q="" page={1} initial={chunk(1, ["a"])} />));
  expect(ids()).toEqual(["a", "b"]);
});
