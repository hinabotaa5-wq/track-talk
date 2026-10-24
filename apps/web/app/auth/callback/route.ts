import { callbackExchangeError, callbackParamsError } from "@/lib/auth-callback";
import { nextCookiePath } from "@/lib/safe-next";
import { createServerClient } from "@supabase/ssr";
import { NextResponse, type NextRequest } from "next/server";

function finish(response: NextResponse) {
  response.cookies.set("tracktalk-next", "", { path: "/", maxAge: 0, sameSite: "lax" });
  return response;
}

function redirectWithinSite(destination: URL) {
  // Netlify 内部のホスト名を Location に出さず、ブラウザが開いているサイト内で移動する。
  // 正規化でパスが // 始まりになった場合も、外部ホストとして解釈されないよう拒む。
  const location = destination.pathname.startsWith("//")
    ? "/"
    : `${destination.pathname}${destination.search}${destination.hash}`;
  return new NextResponse(null, { status: 307, headers: { Location: location } });
}

export async function GET(request: NextRequest) {
  const origin = request.nextUrl.origin;
  const login = new URL("/login", origin);
  const url = process.env.SUPABASE_INTERNAL_URL ?? process.env.NEXT_PUBLIC_SUPABASE_URL;
  const key = process.env.NEXT_PUBLIC_SUPABASE_ANON_KEY;

  if (!url || !key) {
    login.searchParams.set("error", "config");
    return finish(redirectWithinSite(login));
  }

  const failed = callbackParamsError(request.nextUrl.searchParams);
  const code = request.nextUrl.searchParams.get("code");
  if (failed || !code) {
    login.searchParams.set("error", failed ?? "google");
    return finish(redirectWithinSite(login));
  }

  const dest = new URL("/", origin);
  const next = new URL(nextCookiePath(request.cookies.get("tracktalk-next")?.value), origin);
  if (next.origin === origin) {
    // Cookie からはパス・クエリ・フラグメントだけを使い、リダイレクト先の origin は固定する。
    dest.pathname = next.pathname;
    dest.search = next.search;
    dest.hash = next.hash;
  }
  const success = redirectWithinSite(dest);
  const supabase = createServerClient(url, key, {
    ...(process.env.SUPABASE_COOKIE_NAME
      ? { cookieOptions: { name: process.env.SUPABASE_COOKIE_NAME } }
      : {}),
    cookies: {
      getAll() {
        return request.cookies.getAll();
      },
      setAll(cookiesToSet, headers) {
        cookiesToSet.forEach(({ name, value, options }) => {
          success.cookies.set(name, value, { ...options, sameSite: "lax" });
        });
        Object.entries(headers).forEach(([name, value]) => {
          success.headers.set(name, value);
        });
      },
    },
  });

  const { error } = await supabase.auth.exchangeCodeForSession(code);
  if (error) {
    login.searchParams.set("error", callbackExchangeError);
    return finish(redirectWithinSite(login));
  }
  return finish(success);
}
