import type { ApiError } from "./types";

// 開発中のブラウザは /api の書き換えを通す。サーバーで描くときは相対パスを解決できないので、API を直に呼ぶ。
const API_BASE =
  process.env.NODE_ENV === "development" && typeof window !== "undefined"
    ? "/api"
    : (process.env.NEXT_PUBLIC_API_BASE_URL ?? "http://localhost:8080");

const unknownError = "処理に失敗しました。しばらくしてからもう一度試してください。";

function isApiError(value: unknown): value is ApiError {
  if (!value || typeof value !== "object" || !("error" in value)) return false;
  const error = (value as ApiError).error;
  return !!error && typeof error.code === "string" && typeof error.message === "string";
}

export class ApiRequestError extends Error {
  status: number;
  code: string;
  fields?: Record<string, string>;

  constructor(status: number, body: ApiError | string) {
    if (typeof body === "string") {
      super(body);
      this.status = status;
      this.code = "unknown";
      return;
    }
    super(body.error.message);
    this.status = status;
    this.code = body.error.code;
    this.fields = body.error.fields;
  }
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
      cache: "no-store",
    });
  } catch {
    throw new ApiRequestError(0, {
      error: { code: "network", message: "通信に失敗しました。接続を確認してください。" },
    });
  }
  if (res.status === 204) {
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
      isApiError(data) ? data : { error: { code: "unknown", message: unknownError } },
    );
  }
  return data as T;
}
