import { createElement, type ReactNode } from "react";
import { renderToStaticMarkup } from "react-dom/server";
import { describe, expect, it, vi } from "vitest";
import { HomeFeed } from "./home-feed";
import type { Post } from "@/lib/types";

vi.mock("next/link", () => ({
  default: ({ children, href }: { children: ReactNode; href: string }) => createElement("a", { href }, children),
}));

const post = {
  id: "4020b4de-956e-4819-97e6-90cb82f83fc0",
  title: "いい感じ",
  mine: false,
  like_count: 0,
  comment_count: 0,
  liked_by_me: false,
  saved_by_me: false,
  kind: "text",
  created_at: "2026-10-12T00:00:00Z",
  updated_at: "2026-10-12T00:00:00Z",
} as Post;

describe("HomeFeed", () => {
  it("shows a loading message until a failed load can be retried", () => {
    const html = renderToStaticMarkup(
      createElement(HomeFeed, { sort: "popular", q: "", page: 1, initial: null }),
    );
    expect(html).toContain("読み込み中…");
    expect(html).not.toContain("投稿の取得に失敗しました");
  });

  it("renders posts that were already loaded", () => {
    const html = renderToStaticMarkup(
      createElement(HomeFeed, {
        sort: "popular",
        q: "",
        page: 1,
        initial: { posts: [post], meta: { page: 1, per_page: 20, total: 1 } },
      }),
    );
    expect(html).toContain("いい感じ");
    expect(html).not.toContain("読み込み中…");
  });
});
