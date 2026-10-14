// ログイン後の戻り先。同じサイト内のパスだけ許可し、外部サイトへの誘導を防ぐ。
export function safeNextPath(value: string | null | undefined) {
  if (!value || !value.startsWith("/") || value.startsWith("//") || value.includes("\\")) return "/";
  if (value === "/login" || value.startsWith("/login?")) return "/";
  return value;
}

export function nextCookiePath(value: string | null | undefined) {
  if (!value) return "/";
  try {
    return safeNextPath(decodeURIComponent(value));
  } catch {
    return safeNextPath(value);
  }
}
