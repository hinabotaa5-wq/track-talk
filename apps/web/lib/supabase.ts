import { createBrowserClient } from "@supabase/ssr";

export function createClient() {
  const url = process.env.NEXT_PUBLIC_SUPABASE_URL;
  const key = process.env.NEXT_PUBLIC_SUPABASE_ANON_KEY;
  if (!url || !key) {
    return null;
  }
  // https のときだけ Secure を付ける。http（ローカル開発など）で付けるとセッションを保存できない。
  // SSR で評価されても例外にならないよう window の有無を見る。HttpOnly は付けない（ブラウザ側で Cookie を読むため）。
  const secure = typeof window !== "undefined" && window.location.protocol === "https:";
  return createBrowserClient(url, key, {
    cookieOptions: { secure },
    global: {
      fetch: (input, init) => {
        const requestURL = typeof input === "string" ? input : input instanceof URL ? input.href : input.url;
        // ログアウトのサーバーが応答しなくても、SDK が端末のセッションを消せるようにする。
        if (new URL(requestURL).pathname.endsWith("/auth/v1/logout")) {
          return fetch(input, { ...init, signal: AbortSignal.timeout(5000) });
        }
        return fetch(input, init);
      },
    },
  });
}
