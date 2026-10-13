import { createElement, type ReactNode } from "react";
import { renderToStaticMarkup } from "react-dom/server";
import { describe, expect, it, vi } from "vitest";
import { FeedCard } from "./feed-card";
import type { Post } from "@/lib/types";

vi.mock("next/link", () => ({
  default: ({ children, href }: { children: ReactNode; href: string }) => createElement("a", { href }, children),
}));

const post = {
  id: "4020b4de-956e-4819-97e6-90cb82f83fc0",
  title: "自分の投稿",
  mine: true,
  like_count: 0,
  comment_count: 0,
  liked_by_me: false,
  saved_by_me: false,
  kind: "text",
  created_at: "2026-10-12T00:00:00Z",
  updated_at: "2026-10-12T00:00:00Z",
} as Post;

describe("FeedCard", () => {
  it("keeps a return path when opened from my page", () => {
    const html = renderToStaticMarkup(createElement(FeedCard, { post, returnTo: "me" }));
    expect(html).toContain('href="/posts/4020b4de-956e-4819-97e6-90cb82f83fc0?from=me"');
  });

  it("opens the post directly from other feeds", () => {
    const html = renderToStaticMarkup(createElement(FeedCard, { post }));
    expect(html).toContain('href="/posts/4020b4de-956e-4819-97e6-90cb82f83fc0"');
    expect(html).not.toContain("from=me");
    expect(html).toContain("bg-surface");
    expect(html).toContain("text-foreground");
    expect(html).not.toContain("bg-white");
  });

  it("renders optimized image when thumbnail is present", () => {
    const postWithThumb = {
      ...post,
      thumbnail_url: "https://res.cloudinary.com/demo/image/upload/sample.jpg",
    };
    const html = renderToStaticMarkup(createElement(FeedCard, { post: postWithThumb }));
    expect(html).toContain("res.cloudinary.com");
  });
});
