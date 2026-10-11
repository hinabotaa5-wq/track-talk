"use client";

import Link from "next/link";
import { ChevronLeft } from "lucide-react";
import { useAuth } from "./auth-provider";
import { hidesTabBar } from "./bottom-nav";
import { createClient } from "@/lib/supabase";
import { usePathname, useRouter } from "next/navigation";

export function Header() {
  const { session } = useAuth();
  const router = useRouter();
  const pathname = usePathname();
  const showBack = hidesTabBar(pathname);

  async function logout() {
    const supabase = createClient();
    await supabase?.auth.signOut();
    router.push("/");
    router.refresh();
  }

  function goBack() {
    if (window.history.length > 1) router.back();
    else router.push("/");
  }

  return (
    <header className={`border-b border-zinc-200 bg-white${showBack ? " sticky top-0 z-40" : ""}`}>
      <div className="mx-auto flex max-w-5xl items-center justify-between gap-4 px-4 py-3">
        {showBack ? (
          <button
            type="button"
            onClick={goBack}
            aria-label="戻る"
            className="-ml-2 inline-flex size-11 items-center justify-center rounded-md text-blue-700"
          >
            <ChevronLeft className="size-6" strokeWidth={2.4} aria-hidden />
          </button>
        ) : (
          <Link href="/" className="truncate text-lg font-bold tracking-tight text-blue-700">
            TRACK TALK
          </Link>
        )}
        {!session ? (
          <Link href="/login" className="text-sm text-blue-700 md:hidden">
            ログイン
          </Link>
        ) : null}
        <nav className="hidden items-center gap-3 text-sm md:flex">
          <Link href={session ? "/competitions" : "/login"} className="hover:text-blue-700">
            大会
          </Link>
          {session ? (
            <>
              <Link href="/upload" className="hover:text-blue-700">
                投稿する
              </Link>
              <Link href="/me" className="hover:text-blue-700">
                マイページ
              </Link>
              <button type="button" onClick={logout} className="text-zinc-500 hover:text-zinc-800">
                ログアウト
              </button>
            </>
          ) : (
            <Link href="/login" className="hover:text-blue-700">
              ログイン
            </Link>
          )}
        </nav>
      </div>
    </header>
  );
}
