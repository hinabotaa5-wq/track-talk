import { callbackExchangeError, callbackParamsError } from "@/lib/auth-callback";
import { nextCookiePath } from "@/lib/safe-next";
import { createServerClient } from "@supabase/ssr";
import { NextResponse, type NextRequest } from "next/server";

function finish(response: NextResponse) {
  response.cookies.set("tracktalk-next", "", { path: "/", maxAge: 0 });
  return response;
}

export async function GET(request: NextRequest) {
  const origin = request.nextUrl.origin;
  const login = new URL("/login", origin);
  const url = process.env.SUPABASE_INTERNAL_URL ?? process.env.NEXT_PUBLIC_SUPABASE_URL;
  const key = process.env.NEXT_PUBLIC_SUPABASE_ANON_KEY;

  if (!url || !key) {
    login.searchParams.set("error", "config");
    return finish(NextResponse.redirect(login));
  }

  const failed = callbackParamsError(request.nextUrl.searchParams);
  const code = request.nextUrl.searchParams.get("code");
  if (failed || !code) {
    login.searchParams.set("error", failed ?? "google");
    return finish(NextResponse.redirect(login));
  }

  const dest = new URL(nextCookiePath(request.cookies.get("tracktalk-next")?.value), origin);
  if (dest.origin !== origin) {
    dest.href = new URL("/", origin).href;
  }
  const success = NextResponse.redirect(dest);
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
          success.cookies.set(name, value, options);
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
    return finish(NextResponse.redirect(login));
  }
  return finish(success);
}
