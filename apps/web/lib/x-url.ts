// 投稿IDだけを保存し、共有時の追跡パラメータやユーザー名の違いを除く。
export function parseXPostUrl(value: string): string | null {
  try {
    const url = new URL(value.trim());
    if (!["https:", "http:"].includes(url.protocol) || url.username || url.password || url.port) return null;
    if (!["x.com", "www.x.com", "mobile.x.com", "twitter.com", "www.twitter.com", "mobile.twitter.com"].includes(url.hostname.toLowerCase())) return null;
    return /^\/(?:[A-Za-z0-9_]{1,15}\/status|i\/web\/status|i\/status)\/([0-9]{1,25})(?:\/(?:photo|video)\/[1-4])?\/?$/.exec(url.pathname)?.[1] ?? null;
  } catch { return null; }
}

export function xPostPermalink(id: string): string | null {
  return /^[0-9]{1,25}$/.test(id) ? `https://x.com/i/status/${id}` : null;
}
