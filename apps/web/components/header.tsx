"use client";

import Link from "next/link";
import { useAuth } from "./auth-provider";
import { createClient } from "@/lib/supabase";
import { useRouter } from "next/navigation";

export function Header() {
  const { session, profile } = useAuth();
  const router = useRouter();

  async function logout() {
    const supabase = createClient();
    await supabase?.auth.signOut();
    router.push("/");
    router.refresh();
  }

  return (
    <header className="border-b border-zinc-200 bg-white">
      <div className="mx-auto flex max-w-5xl items-center justify-between gap-4 px-4 py-3">
        <Link href="/" className="text-lg font-bold tracking-tight text-orange-700">
          TRACK TALK
        </Link>
        {!session ? (
          <Link href="/login" className="text-sm text-orange-700 md:hidden">
            ログイン
          </Link>
        ) : null}
        <nav className="hidden items-center gap-3 text-sm md:flex">
          <Link href="/posts" className="hover:text-orange-700">
            投稿
          </Link>
          <Link href="/competitions" className="hover:text-orange-700">
            大会
          </Link>
          {session ? (
            <>
              <Link href="/upload" className="hover:text-orange-700">
                投稿する
              </Link>
              <Link href="/me" className="hover:text-orange-700">
                {profile?.nickname ?? "マイページ"}
              </Link>
              <button type="button" onClick={logout} className="text-zinc-500 hover:text-zinc-800">
                ログアウト
              </button>
            </>
          ) : (
            <Link href="/login" className="hover:text-orange-700">
              ログイン
            </Link>
          )}
        </nav>
      </div>
    </header>
  );
}
