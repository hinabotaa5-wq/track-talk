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

  it("opens a video post and uses its details instead of a title", () => {
    const html = renderToStaticMarkup(
      createElement(FeedCard, {
        post: {
          ...post,
          kind: "video",
          title: "",
          excerpt: "走った動画の詳細",
          playback_url: "https://example.com/v.mp4",
          thumbnail_url: "https://example.com/t.jpg",
        },
      }),
    );
    const postLinks = html.match(/href="\/posts\/[^"]+"/g);
    expect(postLinks).toEqual(['href="/posts/4020b4de-956e-4819-97e6-90cb82f83fc0"']);
    expect(html).not.toContain("play=1");
    expect(html).toContain("走った動画の詳細");
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
    expect(noTitle).toContain("投稿の詳細を開く");
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

it("投稿動画のサムネイルは再生せずに詳細を開く", () => {
  const html = renderToStaticMarkup(createElement(FeedCard, { post: { ...post, kind: "video", playback_url: "https://example.com/video.mp4", thumbnail_url: "https://example.com/thumb.jpg" } }));
  expect(html).not.toContain("<video");
  expect(html).toContain(`href="/posts/${post.id}"`);
  expect(html).toContain('src="https://example.com/thumb.jpg"');
  expect(html).toContain('loading="lazy"');
  expect(html).toContain("h-auto w-full");
});

it("YouTubeの縦動画と横動画をそれぞれの比率で埋め込む", () => {
  for (const shorts of [true, false]) {
    const html = renderToStaticMarkup(createElement(FeedCard, { post: { ...post, kind: "youtube", youtube_video_id: "abcdefghijk", youtube_shorts: shorts } }));
    expect(html).toContain("https://www.youtube-nocookie.com/embed/abcdefghijk");
    expect(html).toContain(shorts ? "aspect-[9/16]" : "aspect-video");
    expect(html).not.toMatch(/<a\b[^>]*><iframe/);
  }
});


it("ホームでは再生プレーヤーではなく元のサムネイルを詳細リンクとして表示する", () => {
  const html = renderToStaticMarkup(createElement(FeedCard, {
    post: { ...post, kind: "video", playback_url: "/samples/vertical.mp4", thumbnail_url: "/samples/vertical.jpg" },
    thumbnailOnly: true,
  }));
  expect(html).toContain('/samples/vertical.jpg');
  expect(html).toContain(`/posts/${post.id}`);
  expect(html).not.toContain('<video');
  expect(html).not.toContain('<iframe');
});

it("写真のない文章は本文を一度だけ出し、下のタイトル欄を出さない", () => {
  const html = renderToStaticMarkup(createElement(FeedCard, { post: { ...post, excerpt: "今日の練習の感覚" } }));
  expect(html.match(/今日の練習の感覚/g)).toHaveLength(1);
  expect(html).not.toContain("feed-card-title");
  expect(html).not.toContain("自分の投稿");
});
