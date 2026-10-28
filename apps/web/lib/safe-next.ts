// ログイン後の戻り先。同じサイト内のパスだけ許可し、外部サイトへの誘導を防ぐ。
// URL の解釈ではタブや改行が取り除かれる(`/\t/evil.example` は `//evil.example` になる)ので、制御文字も拒む。
export function safeNextPath(value: string | null | undefined) {
  if (!value || !value.startsWith("/") || value.startsWith("//") || value.includes("\\")) return "/";
  if ([...value].some((c) => c.charCodeAt(0) < 0x20 || c.charCodeAt(0) === 0x7f)) return "/";
  if (value === "/login" || value.startsWith("/login?")) return "/";
  return value;
}

// ログイン画面のパス。ログインしたら、いま開いている画面に戻す。
export function loginPathFrom(location: { pathname: string; search: string }) {
  return `/login?next=${encodeURIComponent(`${location.pathname}${location.search}`)}`;
}

export function rememberNextPath(path: string, maxAge = 600) {
  document.cookie = `tracktalk-next=${encodeURIComponent(safeNextPath(path))}; Path=/; Max-Age=${maxAge}; SameSite=Lax`;
}

export function nextCookiePath(value: string | null | undefined) {
  if (!value) return "/";
  try {
    return safeNextPath(decodeURIComponent(value));
  } catch {
    return safeNextPath(value);
  }
}
