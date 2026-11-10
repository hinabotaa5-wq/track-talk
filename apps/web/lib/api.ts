import { currentLocale, hasEnglish, L } from "@/lib/i18n";
import type { ApiError } from "./types";
import { trackSuccessfulMutation } from "./analytics";

// 開発中のブラウザは /api の書き換えを通す。サーバーで描くときは相対パスを解決できないので、API を直に呼ぶ。
const API_BASE =
  process.env.NODE_ENV === "development" && typeof window !== "undefined"
    ? "/api"
    : ((typeof window === "undefined" ? process.env.API_INTERNAL_URL : undefined)
      ?? process.env.NEXT_PUBLIC_API_BASE_URL ?? "http://localhost:8080");

const unknownError = "処理に失敗しました。しばらくしてからもう一度試してください。";

function isApiError(value: unknown): value is ApiError {
  if (!value || typeof value !== "object" || !("error" in value)) return false;
  const error = (value as ApiError).error;
  return !!error && typeof error.code === "string" && typeof error.message === "string";
}

// API のエラー文（日本語）を表示言語に合わせる（本体 #530）。英語の辞書に無い文は、日本語を混ぜずに一般的な英語の文にする。
function localizedApiMessage(message: string, status: number): string {
  if (currentLocale() !== "en" || hasEnglish(message)) return L(message);
  if (status === 0) return L("通信に失敗しました。接続を確認してください。");
  if (status === 401) return "Please log in again.";
  if (status === 403) return "You can't do this.";
  if (status === 404) return "Not found. It may have been deleted.";
  if (status === 409) return "This has already changed. Reload the page and try again.";
  if (status === 429) return "Too many requests. Please wait a moment and try again.";
  if (status >= 400 && status < 500) return "This can't be done. Please check your input and try again.";
  return L(unknownError);
}

export class ApiRequestError extends Error {
  status: number;
  code: string;
  fields?: Record<string, string>;

  constructor(status: number, body: ApiError | string) {
    if (typeof body === "string") {
      super(localizedApiMessage(body, status));
      this.status = status;
      this.code = "unknown";
      return;
    }
    super(localizedApiMessage(body.error.message, status));
    this.status = status;
    this.code = body.error.code;
    this.fields = body.error.fields
      ? Object.fromEntries(Object.entries(body.error.fields).map(([key, text]) => [key, localizedApiMessage(text, status)]))
      : undefined;
  }
}

// 画面に出すエラーの文言。項目ごとの指摘があれば最初の1つ、なければ API のメッセージ、API 以外のエラーは fallback。
export function apiErrorText(err: unknown, fallback: string): string {
  if (err instanceof ApiRequestError) {
    const field = err.fields ? Object.values(err.fields)[0] : "";
    return field || err.message;
  }
  return fallback;
}

export async function apiFetch<T>(
  path: string,
  init: RequestInit & { token?: string | null } = {},
): Promise<T> {
  const { token, headers, ...rest } = init;
  let res: Response;
  try {
    res = await fetch(`${API_BASE}${path}`, {
      ...rest,
      headers: {
        Accept: "application/json",
        ...(rest.body ? { "Content-Type": "application/json" } : {}),
        ...(token ? { Authorization: `Bearer ${token}` } : {}),
        ...headers,
      },
      // ふだんは毎回取りにいく。サーバーで描く公開の一覧だけ、next.revalidate を渡して取り置きを使える。
      ...(rest.next?.revalidate !== undefined ? {} : { cache: "no-store" as const }),
    });
  } catch {
    throw new ApiRequestError(0, {
      error: { code: "network", message: L("通信に失敗しました。接続を確認してください。") },
    });
  }
  if (res.status === 204) {
    trackSuccessfulMutation(path, rest.method);
    return undefined as T;
  }
  const text = await res.text();
  let data: unknown = null;
  if (text) {
    try {
      data = JSON.parse(text);
    } catch {
      data = null;
    }
  }
  if (!res.ok) {
    throw new ApiRequestError(
      res.status,
      isApiError(data) ? data : { error: { code: "unknown", message: L(unknownError) } },
    );
  }
  trackSuccessfulMutation(path, rest.method);
  return data as T;
}

// apiDownloadText は、テキストを返す API（やりとりのダウンロードなど）を読む。失敗したら ApiRequestError。
export async function apiDownloadText(path: string, token: string | null): Promise<string> {
  let res: Response;
  try {
    res = await fetch(`${API_BASE}${path}`, { headers: token ? { Authorization: `Bearer ${token}` } : {}, cache: "no-store" });
  } catch {
    throw new ApiRequestError(0, { error: { code: "network", message: L("通信に失敗しました。接続を確認してください。") } });
  }
  const text = await res.text();
  if (!res.ok) {
    let data: unknown = null;
    try {
      data = JSON.parse(text);
    } catch {
      data = null;
    }
    const body = data as ApiError | null;
    throw new ApiRequestError(res.status, body?.error ? body : L("ダウンロードできませんでした"));
  }
  return text;
}
