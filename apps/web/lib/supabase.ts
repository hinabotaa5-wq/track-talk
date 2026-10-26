import { createBrowserClient } from "@supabase/ssr";

export function createClient() {
  const url = process.env.NEXT_PUBLIC_SUPABASE_URL;
  const key = process.env.NEXT_PUBLIC_SUPABASE_ANON_KEY;
  if (!url || !key) {
    return null;
  }
  return createBrowserClient(url, key, {
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
