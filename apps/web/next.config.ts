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
    "img-src 'self' data: blob: https://res.cloudinary.com https://i.ytimg.com https://img.youtube.com https://upload.wikimedia.org https://thumb.wikimedia.org",
    "media-src 'self' blob: https://res.cloudinary.com",
    "font-src 'self'",
    "frame-src https://www.youtube-nocookie.com https://www.instagram.com",
    `connect-src ${connect.join(" ")}`,
  ];
  return directives.join("; ");
}

const nextConfig: NextConfig = {
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
