const displayWidth = 960;
const imageHosts = new Set(["upload.wikimedia.org", "thumb.wikimedia.org"]);

// Wikimedia は 1200px を頼むと 1280px のファイルを返す。カードには 960 あれば足り、
// これは CDN がもともと持っているサイズの1つ。
export function cityPhotoSrc(raw: string): string {
  let parsed: URL;
  try {
    parsed = new URL(raw);
  } catch {
    return raw;
  }
  if (parsed.protocol !== "https:" || !imageHosts.has(parsed.hostname)) return raw;

  const parts = parsed.pathname.split("/");
  if (parts[1] !== "wikipedia" || !parts[2]) return raw;

  const last = parts[parts.length - 1] ?? "";
  if (parts[3] === "thumb") {
    const width = last.match(/^(\d+)px-/);
    if (!width) return raw;
    if (Number(width[1]) > displayWidth) {
      parts[parts.length - 1] = `${displayWidth}px-${last.slice(width[0].length)}`;
      parsed.pathname = parts.join("/");
    }
    parsed.search = "";
    parsed.hash = "";
    return parsed.toString();
  }

  if (parts.length !== 6 || !/\.(?:jpe?g|png|webp)$/i.test(last)) return raw;
  parsed.pathname = ["", "wikipedia", parts[2], "thumb", parts[3], parts[4], last, `${displayWidth}px-${last}`].join("/");
  parsed.search = "";
  parsed.hash = "";
  return parsed.toString();
}

const warmed = new Set<string>();

export function preloadCityPhotos(urls: string[], priority: "high" | "low" = "low") {
  if (typeof window === "undefined") return;
  for (const url of urls) {
    const src = cityPhotoSrc(url);
    if (!src || warmed.has(`${priority}:${src}`)) continue;
    warmed.add(`${priority}:${src}`);
    const img = new Image();
    img.decoding = "async";
    img.fetchPriority = priority;
    img.src = src;
  }
}
