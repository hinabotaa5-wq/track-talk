import { createElement, type ReactNode } from "react";
import { renderToStaticMarkup } from "react-dom/server";
import { describe, expect, it, vi } from "vitest";
import { FeedCard } from "./feed-card";
import type { Post } from "@/lib/types";

vi.mock("next/link", () => ({
  default: ({ children, href, ...rest }: { children: ReactNode; href: string }) => createElement("a", { href, ...rest }, children),
}));

vi.mock("next/navigation", () => ({
  useRouter: () => ({ push: () => {} }),
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
    expect(html).not.toContain("play=1");
  });

  it("hides the like mark on the list card", () => {
    const html = renderToStaticMarkup(createElement(FeedCard, { post: { ...post, like_count: 4 } }));
    expect(html).not.toContain("いいね");
    expect(html).not.toContain(">4<");
  });

  it("plays a video from the thumbnail and opens the title without playback", () => {
    const html = renderToStaticMarkup(
      createElement(FeedCard, {
        post: {
          ...post,
          kind: "video",
          playback_url: "https://example.com/v.mp4",
          thumbnail_url: "https://example.com/t.jpg",
        },
      }),
    );
    const hrefs = html.match(/href="[^"]+"/g);
    expect(hrefs).toContain('href="/posts/4020b4de-956e-4819-97e6-90cb82f83fc0?play=1"');
    expect(hrefs).toContain('href="/posts/4020b4de-956e-4819-97e6-90cb82f83fc0"');
    expect(html).toContain('aria-label="自分の投稿を再生"');
  });

  it("keeps the my-page return path on a playing thumbnail", () => {
    const html = renderToStaticMarkup(
      createElement(FeedCard, {
        post: { ...post, kind: "youtube", youtube_video_id: "abcdefghijk" },
        returnTo: "me",
      }),
    );
    const hrefs = html.match(/href="[^"]+"/g);
    expect(hrefs).toContain('href="/posts/4020b4de-956e-4819-97e6-90cb82f83fc0?from=me&amp;play=1"');
    expect(hrefs).toContain('href="/posts/4020b4de-956e-4819-97e6-90cb82f83fc0?from=me"');
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
