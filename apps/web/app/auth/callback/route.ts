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
  const url = process.env.NEXT_PUBLIC_SUPABASE_URL;
  const key = process.env.NEXT_PUBLIC_SUPABASE_ANON_KEY;

  if (!url || !key) {
    login.searchParams.set("error", "config");
    return finish(NextResponse.redirect(login));
  }

  const code = request.nextUrl.searchParams.get("code");
  const oauthError =
    request.nextUrl.searchParams.get("error_description") ??
    request.nextUrl.searchParams.get("error");
  if (!code || oauthError) {
    login.searchParams.set("error", "google");
    return finish(NextResponse.redirect(login));
  }

  const success = NextResponse.redirect(new URL(nextCookiePath(request.cookies.get("tracktalk-next")?.value), origin));
  const supabase = createServerClient(url, key, {
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
    login.searchParams.set("error", "google");
    return finish(NextResponse.redirect(login));
  }
  return finish(success);
}
