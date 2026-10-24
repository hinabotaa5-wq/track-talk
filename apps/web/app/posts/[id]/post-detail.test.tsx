/** @vitest-environment jsdom */
import { createElement } from "react";
import { renderToStaticMarkup } from "react-dom/server";
import { expect, it, vi } from "vitest";
import { PostDetail } from "./post-detail";
import type { Comment, Post } from "@/lib/types";
let room = "";
vi.mock("@/components/auth-provider", () => ({ useAuth: () => ({ session: null, token: null }) }));
vi.mock("next/navigation", () => ({ useRouter: () => ({}), useSearchParams: () => new URLSearchParams(room) }));
vi.mock("@/components/confirm-dialog", () => ({ useConfirm: () => vi.fn(), reportConfirmOptions: () => ({}) }));
const post = { id: "post-1", kind: "video", title: "走りの動画", body: "スタートで上体が起きてしまいます。改善点を教えてください。", playback_url: "/samples/vertical.mp4", thumbnail_url: "/samples/vertical.jpg", mine: false, like_count: 0, comment_count: 0, liked_by_me: false, saved_by_me: false, created_at: "2026-10-24T00:00:00Z", updated_at: "2026-10-24T00:00:00Z" } as Post;
function render(current = post, comments: Comment[] = []) {
  const container = document.createElement("div");
  container.innerHTML = renderToStaticMarkup(createElement(PostDetail, { id: current.id, initial: { post: current, comments } }));
  return container;
}
it.each(["video", "youtube", "instagram"] as const)("%s の詳細文を動画より上に普通のサイズで表示し枠を付けない", (kind) => {
  room = "";
  const container = render({ ...post, kind, youtube_video_id: "abcdefghijk", instagram_type: "reel", instagram_shortcode: "C1a2B3c4D5e" });
  const description = container.querySelector(".post-description")!;
  const media = container.querySelector("video, iframe")!;
  expect(description.textContent).toBe(post.body);
  expect(description.classList.contains("text-base")).toBe(true);
  expect(description.className).not.toMatch(/border|rounded|glass|bg-/);
  expect(description.compareDocumentPosition(media) & Node.DOCUMENT_POSITION_FOLLOWING).toBeTruthy();
});
it("チャットの長い連続文字を折り返し、小さい文字で全文表示する", () => {
  room = "room=comment-1";
  const body = "A".repeat(600);
  const comment = { id: "comment-1", body, parent_id: null, reply_to_id: null, author_name: "匿名", mine: false, created_at: "2026-10-24T00:00:00Z" } as Comment;
  const container = render(post, [comment]);
  const bubble = container.querySelector("#comment-comment-1 p")!;
  expect(bubble.textContent).toBe(body);
  expect(bubble.classList.contains("text-xs")).toBe(true);
  expect(bubble.className).toContain("[overflow-wrap:anywhere]");
  expect(bubble.className).not.toMatch(/truncate|line-clamp/);
  room = "";
});
