import type { NextConfig } from "next";

function origin(raw: string | undefined) {
  if (!raw) return [];
  try {
    return [new URL(raw).origin];
  } catch {
    return [];
  }
}

function contentSecurityPolicy() {
  const api = process.env.NEXT_PUBLIC_API_BASE_URL ?? "http://localhost:8080";
  const connect = [
    "'self'",
    ...origin(api),
    ...origin(process.env.NEXT_PUBLIC_SUPABASE_URL),
    "https://*.supabase.co",
    "wss://*.supabase.co",
    "https://api.cloudinary.com",
    "https://res.cloudinary.com",
    "https://cloud.umami.is",
    "https://gateway.umami.is",
    ...origin(process.env.NEXT_PUBLIC_POSTHOG_HOST),
  ];
  const script = ["'self'", "'unsafe-inline'", "https://cloud.umami.is"];
  if (process.env.NODE_ENV !== "production") script.push("'unsafe-eval'");
  const directives = [
    "default-src 'self'",
    "base-uri 'self'",
    "object-src 'none'",
    "frame-ancestors 'none'",
    "form-action 'self'",
    `script-src ${script.join(" ")}`,
    "style-src 'self' 'unsafe-inline'",
    "img-src 'self' data: blob: https://res.cloudinary.com https://i.ytimg.com https://img.youtube.com https://upload.wikimedia.org https://thumb.wikimedia.org https://www.instagram.com https://*.cdninstagram.com https://*.fbcdn.net",
    "media-src 'self' blob: https://res.cloudinary.com",
    "font-src 'self'",
    "frame-src https://www.youtube-nocookie.com https://www.instagram.com",
    `connect-src ${connect.join(" ")}`,
  ];
  return directives.join("; ");
}

const nextConfig: NextConfig = {
  async rewrites() {
    const apiBase = process.env.NEXT_PUBLIC_API_BASE_URL ?? "http://localhost:8080";
    if (process.env.NODE_ENV === "production") return [];
    return [
      {
        source: "/api/:path*",
        destination: `${apiBase}/:path*`,
      },
    ];
  },
  async headers() {
    return [
      {
        source: "/:path*",
        headers: [
          { key: "Referrer-Policy", value: "no-referrer" },
          { key: "X-Frame-Options", value: "DENY" },
          { key: "X-Content-Type-Options", value: "nosniff" },
          { key: "Content-Security-Policy", value: contentSecurityPolicy() },
        ],
      },
      // ホームはログインに関係なく同じ中身なので、Netlify の端で短くキャッシュする。
      // 関数がオハイオで動くため、毎回描くと東京の API との往復で遅い。少し古くても画面側が開いたあとに取り直す。
      // Next.js の Cache-Control は上書きされるので Netlify 専用のヘッダーで指定する。
      // Netlify の Next.js アダプターはクエリをキャッシュの区別に入れないため、並び替え・検索・ページを Netlify-Vary で足す。
      {
        source: "/",
        headers: [
          {
            key: "Netlify-CDN-Cache-Control",
            value: "public, durable, max-age=60, stale-while-revalidate=3600",
          },
          { key: "Netlify-Vary", value: "query=sort|q|page" },
        ],
      },
      // 大会の日程と県ページも、ログインに関係なく同じ中身なので端で短くキャッシュする(ホームと同じ理由)。
      // 都道府県の大会(/competitions/regional)は Cookie で県を選ぶので、ここには入れない。
      {
        source: "/competitions",
        headers: [
          {
            key: "Netlify-CDN-Cache-Control",
            value: "public, durable, max-age=300, stale-while-revalidate=3600",
          },
          { key: "Netlify-Vary", value: "query=view|month|prefecture" },
        ],
      },
      // 投稿の詳細も、サーバーはログインなしで読める中身だけを描く(いいね状態などは画面側が開いたあとに読み直す)。
      // 毎回描くとオハイオの関数から東京の API へ2本往復してから画面が開くので、端で短くキャッシュする。
      // 消された・非表示にされた投稿を長く出し続けないよう、ホームより短くする。作成画面(/posts/new など)は ID の形で外す。
      {
        source: "/posts/:id([0-9a-fA-F]{8}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{12})",
        headers: [
          {
            key: "Netlify-CDN-Cache-Control",
            value: "public, durable, max-age=60, stale-while-revalidate=300",
          },
          { key: "Netlify-Vary", value: "query=from|room" },
        ],
      },
      {
        source: "/competitions/major-hosts",
        headers: [{ key: "Netlify-CDN-Cache-Control", value: "public, durable, max-age=300, stale-while-revalidate=3600" }],
      },
      {
        source: "/prefectures/:code",
        headers: [{ key: "Netlify-CDN-Cache-Control", value: "public, durable, max-age=300, stale-while-revalidate=3600" }],
      },
    ];
  },
  async redirects() {
    return [
      { source: "/about", destination: "/guidelines", permanent: false },
      { source: "/posts", destination: "/", permanent: false },
    ];
  },
  turbopack: {
    root: __dirname,
    rules: {
      "*.css": {
        loaders: ["@tailwindcss/turbopack"],
        as: "*.css",
      },
    },
  },
  images: {
    remotePatterns: [
      { protocol: "https", hostname: "res.cloudinary.com" },
      { protocol: "https", hostname: "i.ytimg.com" },
      { protocol: "https", hostname: "img.youtube.com" },
      { protocol: "https", hostname: "upload.wikimedia.org" },
      { protocol: "https", hostname: "thumb.wikimedia.org" },
    ],
  },
};

export default nextConfig;
