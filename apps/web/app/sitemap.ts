import type { MetadataRoute } from "next";
import { apiFetch } from "@/lib/api";
import { injuryBodyParts } from "@/lib/injury";
import { siteUrl } from "@/lib/site";
import type { Meta, Post } from "@/lib/types";

// 1時間ごとに作り直す。API が落ちていても固定ページだけは返す。
export const revalidate = 3600;

// ケガ部のグループページはログインなしで読める集計ページなので載せる(会話は参加者だけ)。
const staticPages = [
  "/",
  "/injury",
  ...injuryBodyParts.map((part) => `/injury/${part.value}`),
  "/injury/stats",
  "/coaching",
  "/coaches",
  "/guidelines",
  "/terms",
  "/privacy",
];

async function recentPosts(): Promise<Post[]> {
  // ログインなしで読める投稿だけを載せる(ケガの投稿はグループ参加者しか読めないので一覧に出てこない)。
  const pages = await Promise.all(
    [1, 2, 3, 4].map((page) =>
      apiFetch<{ posts: Post[]; meta: Meta }>(`/v1/posts?sort=new&page=${page}&per_page=50`, {
        signal: AbortSignal.timeout(5000),
      })
        .then((data) => data.posts ?? [])
        .catch(() => []),
    ),
  );
  return pages.flat().filter((post) => post.kind !== "injury");
}

export default async function sitemap(): Promise<MetadataRoute.Sitemap> {
  const posts = await recentPosts();
  return [
    ...staticPages.map((path) => ({ url: `${siteUrl}${path === "/" ? "" : path}` })),
    ...posts.map((post) => ({
      url: `${siteUrl}/posts/${post.id}`,
      lastModified: post.updated_at,
    })),
  ];
}
