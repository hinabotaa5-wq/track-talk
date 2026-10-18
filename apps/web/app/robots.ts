import type { MetadataRoute } from "next";
import { siteUrl } from "@/lib/site";

// ログインが必要なページや個人のページは検索結果に出さない(大会の日程と県ページはログインなしで読めるので載せる)。
export default function robots(): MetadataRoute.Robots {
  return {
    rules: {
      userAgent: "*",
      allow: "/",
      disallow: ["/me", "/admin", "/upload", "/auth", "/posts/new", "/posts/*/edit", "/injury/*/new"],
    },
    sitemap: `${siteUrl}/sitemap.xml`,
    host: siteUrl,
  };
}
