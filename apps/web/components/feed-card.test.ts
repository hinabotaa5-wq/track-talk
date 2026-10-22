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
  });

  it("hides the like mark on the list card", () => {
    const html = renderToStaticMarkup(createElement(FeedCard, { post: { ...post, like_count: 4 } }));
    expect(html).not.toContain("いいね");
    expect(html).not.toContain(">4<");
  });

  it("opens a video post from the thumbnail and the title", () => {
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
    const postLinks = html.match(/href="\/posts\/[^"]+"/g);
    expect(postLinks).toEqual(['href="/posts/4020b4de-956e-4819-97e6-90cb82f83fc0"']);
    expect(html).not.toContain("play=1");
  });

  it("keeps the my-page return path on a youtube card", () => {
    const html = renderToStaticMarkup(
      createElement(FeedCard, {
        post: { ...post, kind: "youtube", youtube_video_id: "abcdefghijk" },
        returnTo: "me",
      }),
    );
    const postLinks = html.match(/href="\/posts\/[^"]+"/g);
    expect(postLinks).toEqual(['href="/posts/4020b4de-956e-4819-97e6-90cb82f83fc0?from=me"']);
    expect(html).not.toContain("play=1");
  });

  it("renders optimized image when thumbnail is present", () => {
    const postWithThumb = {
      ...post,
      thumbnail_url: "https://res.cloudinary.com/demo/image/upload/sample.jpg",
    };
    const html = renderToStaticMarkup(createElement(FeedCard, { post: postWithThumb }));
    expect(html).toContain("res.cloudinary.com");
  });

  it("uses the body in place of the title on a text post card", () => {
    const html = renderToStaticMarkup(createElement(FeedCard, { post: { ...post, excerpt: "決勝は向かい風だった。" } }));
    expect(html).toContain("決勝は向かい風だった。");
    expect(html).not.toContain("自分の投稿");
  });

  it("uses the body in place of the title on a url post card", () => {
    const withTitle = renderToStaticMarkup(
      createElement(FeedCard, { post: { ...post, kind: "youtube", excerpt: "動画についての本文", youtube_video_id: "abcdefghijk", thumbnail_url: "https://i.ytimg.com/vi/x/hq.jpg" } }),
    );
    expect(withTitle).toContain("動画についての本文");
    expect(withTitle).not.toContain("自分の投稿");
    const noTitle = renderToStaticMarkup(
      createElement(FeedCard, { post: { ...post, title: "", kind: "youtube", youtube_video_id: "abcdefghijk", thumbnail_url: "https://i.ytimg.com/vi/x/hq.jpg" } }),
    );
    expect(noTitle).not.toContain("（質問なし）");
    expect(noTitle).not.toContain("<p ");
  });

  it("does not show a placeholder when a post has no title", () => {
    const html = renderToStaticMarkup(createElement(FeedCard, { post: { ...post, title: "" } }));
    expect(html).not.toContain("質問なし");
  });

  it("loads the cover of an instagram post", () => {
    const html = renderToStaticMarkup(
      createElement(FeedCard, { post: { ...post, kind: "instagram", instagram_type: "reel", instagram_shortcode: "C1a2B3c4D5e" } }),
    );
    expect(html).toContain("https://www.instagram.com/reel/C1a2B3c4D5e/media/?size=m");
  });
});
