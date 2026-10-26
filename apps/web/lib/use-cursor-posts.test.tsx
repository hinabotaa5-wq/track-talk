/** @vitest-environment jsdom */
import { act, useEffect } from "react";
import { createRoot, type Root } from "react-dom/client";
import { afterEach, beforeEach, expect, it, vi } from "vitest";
import { apiFetch, ApiRequestError } from "@/lib/api";
import { cursorPostsPath, useCursorPosts } from "./use-cursor-posts";
import type { Post } from "@/lib/types";

vi.mock("@/lib/api", async (importOriginal) => ({ ...(await importOriginal<typeof import("@/lib/api")>()), apiFetch: vi.fn() }));
const request = vi.mocked(apiFetch);
const page = (ids: string[], next = "") => ({ posts: ids.map((id) => ({ id, title: id, kind: "text" }) as Post), meta: { page: 1, per_page: 24, total: 0, has_more: !!next, next_cursor: next } });

const latest: { current?: ReturnType<typeof useCursorPosts> } = {};
function Probe({ base, token = "token" }: { base: string | null; token?: string }) {
  const feed = useCursorPosts(base, token);
  useEffect(() => { latest.current = feed; });
  return null;
}
const feedOf = () => latest.current!;
let container: HTMLDivElement;
let root: Root;
beforeEach(() => {
  (globalThis as typeof globalThis & { IS_REACT_ACT_ENVIRONMENT: boolean }).IS_REACT_ACT_ENVIRONMENT = true;
  request.mockReset();
  container = document.createElement("div");
  root = createRoot(container);
});
afterEach(() => act(() => root.unmount()));

it("カーソルの URL を組み立てる", () => {
  expect(cursorPostsPath("/v1/me/posts?sort=new")).toBe("/v1/me/posts?sort=new&pagination=cursor&per_page=24");
  expect(cursorPostsPath("/v1/me/posts", "a+b")).toBe("/v1/me/posts?pagination=cursor&per_page=24&cursor=a%2Bb");
});

it("最初の1ページだけ読み、続きは重ならないように足して終わりで止まる", async () => {
  request.mockResolvedValueOnce(page(["1", "2"], "c1")).mockResolvedValueOnce(page(["2", "3"]));
  await act(async () => root.render(<Probe base="/v1/me/posts?sort=new" />));
  expect(request).toHaveBeenCalledTimes(1);
  expect(feedOf().posts?.map((post) => post.id)).toEqual(["1", "2"]);
  expect(feedOf().hasMore).toBe(true);
  await act(async () => { await feedOf().loadMore(); });
  expect(request).toHaveBeenLastCalledWith("/v1/me/posts?sort=new&pagination=cursor&per_page=24&cursor=c1", { token: "token" });
  expect(feedOf().posts?.map((post) => post.id)).toEqual(["1", "2", "3"]);
  expect(feedOf().hasMore).toBe(false);
  await act(async () => { await feedOf().loadMore(); });
  expect(request).toHaveBeenCalledTimes(2);
});

it("期限切れのカーソルは読み直しを促す", async () => {
  request.mockResolvedValueOnce(page(["1"], "old")).mockRejectedValueOnce(new ApiRequestError(400, { error: { code: "invalid_cursor", message: "x" } } as never));
  await act(async () => root.render(<Probe base="/v1/me/posts" />));
  await act(async () => { await feedOf().loadMore(); });
  expect(feedOf().expired).toBe(true);
  expect(feedOf().moreError).not.toBe("");
  expect(feedOf().posts?.map((post) => post.id)).toEqual(["1"]);
});

it("読み込む一覧が決まるまでは取りにいかない", async () => {
  await act(async () => root.render(<Probe base={null} />));
  expect(request).not.toHaveBeenCalled();
  expect(feedOf().posts).toBeNull();
});

it("アカウントが変わると前の人の一覧を表示しない", async () => {
  request.mockResolvedValueOnce(page(["private-user-a"]));
  await act(async () => root.render(<Probe base="/v1/me/saves" token="a" />));
  expect(feedOf().posts?.[0].id).toBe("private-user-a");
  request.mockReturnValueOnce(new Promise(() => {}));
  await act(async () => root.render(<Probe base="/v1/me/saves" token="b" />));
  expect(feedOf().posts).toBeNull();
});
