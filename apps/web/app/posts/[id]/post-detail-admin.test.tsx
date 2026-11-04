/** @vitest-environment jsdom */
import { act, createElement } from "react";
import { createRoot, type Root } from "react-dom/client";
import { afterEach, beforeEach, expect, it, vi } from "vitest";
import { PostDetail } from "./post-detail";
import type { Comment, Post } from "@/lib/types";

const fetch = vi.hoisted(() => vi.fn());
const confirmDialog = vi.hoisted(() => vi.fn());
const router = vi.hoisted(() => ({ push: vi.fn(), refresh: vi.fn(), replace: vi.fn() }));
const auth = vi.hoisted(() => ({ profile: { is_admin: true } as { is_admin: boolean } | null }));
const clearHomeFeeds = vi.hoisted(() => vi.fn());
vi.mock("@/lib/api", async (original) => ({ ...(await original<typeof import("@/lib/api")>()), apiFetch: fetch }));
vi.mock("@/components/auth-provider", () => ({ useAuth: () => ({ session: {}, token: "token", profile: auth.profile }) }));
vi.mock("next/navigation", () => ({ useRouter: () => router, useSearchParams: () => new URLSearchParams(room) }));
vi.mock("@/components/confirm-dialog", async (original) => ({
  ...(await original<typeof import("@/components/confirm-dialog")>()),
  useConfirm: () => confirmDialog,
}));
vi.mock("@/lib/home-feed-cache", async (original) => ({
  ...(await original<typeof import("@/lib/home-feed-cache")>()),
  clearHomeFeeds,
}));

let room = "room=c1";
const post = { id: "post-1", kind: "video", title: "走りの動画", body: "本文", playback_url: "/samples/vertical.mp4", thumbnail_url: "/samples/vertical.jpg", mine: false, like_count: 0, comment_count: 1, liked_by_me: false, saved_by_me: false, created_at: "2026-10-24T00:00:00Z", updated_at: "2026-10-24T00:00:00Z" } as Post;
const comment = { id: "c1", body: "コメント本文", parent_id: null, reply_to_id: null, author_name: "匿名", mine: false, created_at: "2026-10-24T00:00:00Z" } as Comment;

let root: Root, container: HTMLDivElement;
let hiddenComment = false;

function button(text: string) {
  return [...container.querySelectorAll("button")].find((b) => b.textContent === text);
}

async function show() {
  await act(async () => root.render(createElement(PostDetail, { id: post.id, initial: { post, comments: [comment] } })));
}

async function click(text: string) {
  await act(async () => button(text)!.dispatchEvent(new MouseEvent("click", { bubbles: true })));
}

beforeEach(() => {
  (globalThis as typeof globalThis & { IS_REACT_ACT_ENVIRONMENT: boolean }).IS_REACT_ACT_ENVIRONMENT = true;
  auth.profile = { is_admin: true };
  hiddenComment = false;
  room = "room=c1";
  vi.clearAllMocks();
  confirmDialog.mockResolvedValue(true);
  fetch.mockImplementation(async (path: string) => {
    if (path === "/v1/admin/comments/c1/hide") {
      hiddenComment = true;
      return undefined;
    }
    if (path.endsWith("/comments?per_page=50&page=1")) return { comments: hiddenComment ? [] : [comment], meta: { total: hiddenComment ? 0 : 1 } };
    if (path.endsWith("/my-reports")) return { post: false, comment_ids: [] };
    if (path === `/v1/posts/${post.id}`) return post;
    return undefined;
  });
  Element.prototype.scrollIntoView = vi.fn();
  container = document.createElement("div");
  document.body.appendChild(container);
  root = createRoot(container);
});

afterEach(async () => {
  await act(async () => root.unmount());
  container.remove();
});

it("管理者でなければ非表示の操作を出さない", async () => {
  auth.profile = { is_admin: false };
  await show();
  expect(button("非表示")).toBeUndefined();
  await act(async () => root.unmount());
  room = "";
  root = createRoot(container);
  await show();
  expect(button("非表示にする")).toBeUndefined();
});

it("管理者はコメントを非表示にでき、読み直す", async () => {
  await show();
  fetch.mockClear();
  await click("非表示");
  expect(confirmDialog).toHaveBeenCalledTimes(1);
  expect(fetch).toHaveBeenCalledWith("/v1/admin/comments/c1/hide", { method: "POST", token: "token" });
  expect(container.textContent).not.toContain("コメント本文");
});

it("管理者は投稿を非表示にでき、一覧へ戻る", async () => {
  room = "";
  await show();
  await click("非表示にする");
  expect(fetch).toHaveBeenCalledWith(`/v1/admin/posts/${post.id}/hide`, { method: "POST", token: "token" });
  expect(clearHomeFeeds).toHaveBeenCalled();
  expect(router.push).toHaveBeenCalledWith("/");
});

it("確認でやめたら API を呼ばない", async () => {
  confirmDialog.mockResolvedValue(false);
  await show();
  await click("非表示");
  expect(fetch).not.toHaveBeenCalledWith(expect.stringContaining("/hide"), expect.anything());
});
