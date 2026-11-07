// 漏洩済みパスワードの登録ブロック（本体 #429）。
// Have I Been Pwned の Pwned Passwords range API で照合する。
// Supabase の Leaked Password Protection は Pro 限定のため、Free のまま同じ照合を自前で行う。
// k-anonymity のため送るのは SHA-1 の先頭5文字だけで、残りは手元で照合する。
// パスワード・ハッシュ（全体・接頭辞を含む）はログに残さない。

export const pwnedPasswordMessage =
  "このパスワードは漏洩情報に含まれているため使えません。別のパスワードを設定してください。";

// Issue の目安（2〜3秒）の範囲内。登録フローを遅くしない。
export const pwnedCheckTimeoutMs = 2500;

async function sha1HexText(text: string): Promise<string> {
  const bytes = await crypto.subtle.digest("SHA-1", new TextEncoder().encode(text));
  return Array.from(new Uint8Array(bytes))
    .map((b) => b.toString(16).padStart(2, "0"))
    .join("")
    .toUpperCase();
}

// range API の応答（"接尾辞:件数" の改行区切り）に探す接尾辞があるか。切り出してテストする。
export function matchesPwnedRangeBody(body: string, suffix: string): boolean {
  const want = suffix.toUpperCase();
  for (const line of body.split("\n")) {
    const head = line.split(":")[0]?.trim().toUpperCase();
    if (head && head === want) return true;
  }
  return false;
}

type PwnedDeps = {
  fetchFn?: typeof fetch;
  hashFn?: (text: string) => Promise<string>;
  timeoutMs?: number;
};

// コーパスに一致したら true。届かない・エラー・タイムアウト・非200は通す（fail-open）。
export async function isPwnedPassword(password: string, deps: PwnedDeps = {}): Promise<boolean> {
  if (!password) return false;
  const fetchFn = deps.fetchFn ?? fetch;
  const hashFn = deps.hashFn ?? sha1HexText;
  const timeoutMs = deps.timeoutMs ?? pwnedCheckTimeoutMs;
  let hash: string;
  try {
    hash = await hashFn(password);
  } catch {
    return false;
  }
  const prefix = hash.slice(0, 5);
  const suffix = hash.slice(5);
  if (prefix.length !== 5 || suffix.length === 0) return false;
  const controller = new AbortController();
  const timer = setTimeout(() => controller.abort(), timeoutMs);
  try {
    const res = await fetchFn(`https://api.pwnedpasswords.com/range/${prefix}`, {
      headers: { "Add-Padding": "true" },
      signal: controller.signal,
    });
    if (!res.ok) return false;
    const body = await res.text();
    return matchesPwnedRangeBody(body, suffix);
  } catch {
    return false;
  } finally {
    clearTimeout(timer);
  }
}
