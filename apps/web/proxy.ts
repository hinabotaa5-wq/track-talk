import { createServerClient } from "@supabase/ssr";
import { NextResponse, type NextRequest } from "next/server";

// ログインが必要なページは、画面を返す前にサーバー側で弾く。
// 表示してから client 側で /login に飛ばす方式では、中身の枠が一瞬見えてしまう。
export async function proxy(request: NextRequest) {
  const url = process.env.NEXT_PUBLIC_SUPABASE_URL;
  const key = process.env.NEXT_PUBLIC_SUPABASE_ANON_KEY;

  let response = NextResponse.next({ request });
  let claims: unknown = null;

  if (url && key) {
    const supabase = createServerClient(url, key, {
      cookies: {
        getAll() {
          return request.cookies.getAll();
        },
        setAll(cookiesToSet, headers) {
          cookiesToSet.forEach(({ name, value }) => request.cookies.set(name, value));
          response = NextResponse.next({ request });
          cookiesToSet.forEach(({ name, value, options }) =>
            response.cookies.set(name, value, options),
          );
          Object.entries(headers).forEach(([name, value]) => response.headers.set(name, value));
        },
      },
    });
    // Cookie の中身を信用せず、署名を検証する。期限切れならここで更新される。
    const { data } = await supabase.auth.getClaims();
    claims = data?.claims ?? null;
  }

  if (claims) return response;

  const login = new URL("/login", request.nextUrl.origin);
  if (!url || !key) login.searchParams.set("error", "config");
  else login.searchParams.set("next", `${request.nextUrl.pathname}${request.nextUrl.search}`);
  const redirect = NextResponse.redirect(login);
  response.cookies.getAll().forEach((cookie) => redirect.cookies.set(cookie));
  redirect.headers.set("Cache-Control", "no-store");
  return redirect;
}

export const config = {
  matcher: [
    "/upload/:path*",
    "/competitions/:path*",
    "/me/:path*",
    "/posts/new/:path*",
    "/posts/video/:path*",
    "/posts/youtube/:path*",
    "/posts/instagram/:path*",
    "/posts/practice/:path*",
    "/posts/recruit/:path*",
    "/posts/:id/edit",
  ],
};
