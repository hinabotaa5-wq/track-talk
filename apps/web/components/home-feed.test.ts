import { createElement, type ReactNode } from "react";
import { renderToStaticMarkup } from "react-dom/server";
import { describe, expect, it, vi } from "vitest";
import { HomeFeed } from "./home-feed";
import type { Post } from "@/lib/types";

vi.mock("next/link", () => ({
  default: ({ children, href }: { children: ReactNode; href: string }) => createElement("a", { href }, children),
}));

vi.mock("next/navigation", () => ({
  useRouter: () => ({ push: () => {} }),
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
  it("shows a loading frame until a failed load can be retried", () => {
    const html = renderToStaticMarkup(
      createElement(HomeFeed, { sort: "popular", q: "", page: 1, initial: null }),
    );
    expect(html).toContain("投稿を読み込んでいます");
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
    expect(html).not.toContain("投稿を読み込んでいます");
  });

  it("shows club and practice posts with their labels", () => {
    const practice = {
      ...post,
      id: "11111111-1111-4111-8111-111111111111",
      title: "100m 10月20日 18:00",
      kind: "practice",
        practice: {
        venue: "駒沢競技場",
        starts_at: "2026-10-20T09:00:00.000Z",
        event: "100m",
        event_label: "100m",
        capacity: 3,
        accepted_count: 1,
        status: "open",
        reputation: { good: 0, ok: 0, bad: 0 },
      },
    } as Post;
    const club = {
      ...post,
      id: "22222222-2222-4222-8222-222222222222",
      title: "城南クラブ",
      kind: "recruitment",
    } as Post;
    const html = renderToStaticMarkup(
      createElement(HomeFeed, {
        sort: "new",
        q: "",
        page: 1,
        kind: "recruitment",
        initial: { posts: [club, practice], meta: { page: 1, per_page: 20, total: 2 } },
      }),
    );
    expect(html).toContain("クラブ");
    expect(html).toContain("練習");
    expect(html).toContain("駒沢競技場");
    expect(html).toContain("募集 1/3人");
    expect(html).toContain("まだ評価がありません");
  });
});
