export type InstagramType = "p" | "reel";

const instagramHosts = new Set(["instagram.com", "www.instagram.com", "m.instagram.com"]);

export function isInstagramShortcode(value: string) {
  return /^[A-Za-z0-9_-]{5,64}$/.test(value);
}

// API と同じ規則で、公開の投稿(/p/)とリール(/reel/)の URL だけを受け付ける。
export function parseInstagramUrl(value: string): { type: InstagramType; shortcode: string } | null {
  let url: URL;
  try {
    url = new URL(value.trim());
  } catch {
    return null;
  }
  if ((url.protocol !== "https:" && url.protocol !== "http:") || url.username || url.password || url.port) return null;
  if (!instagramHosts.has(url.hostname.toLowerCase().replace(/\.$/, ""))) return null;
  let parts = url.pathname.split("/").filter(Boolean);
  if (parts.length >= 3 && ["p", "reel", "reels"].includes(parts[1])) parts = parts.slice(1);
  if (parts.length < 2) return null;
  const type: InstagramType | null = parts[0] === "p" ? "p" : parts[0] === "reel" || parts[0] === "reels" ? "reel" : null;
  if (!type || !isInstagramShortcode(parts[1])) return null;
  return { type, shortcode: parts[1] };
}

export function instagramPermalink(type: InstagramType, shortcode: string) {
  return `https://www.instagram.com/${type}/${shortcode}/`;
}
