import type { MetadataRoute } from "next";

export default function manifest(): MetadataRoute.Manifest {
  return {
    name: "TrackTalk",
    short_name: "TrackTalk",
    description: "陸上競技を、もっと深く楽しむコミュニティ",
    start_url: "/",
    display: "standalone",
    background_color: "#ffffff",
    theme_color: "#16243a",
    lang: "ja",
    icons: [
      { src: "/icons/tracktalk-192.png", sizes: "192x192", type: "image/png", purpose: "any" },
      { src: "/icons/tracktalk-512.png", sizes: "512x512", type: "image/png", purpose: "any" },
      { src: "/icons/tracktalk-512.png", sizes: "512x512", type: "image/png", purpose: "maskable" },
    ],
  };
}
