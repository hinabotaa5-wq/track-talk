"use client";

import Link from "next/link";
import { usePathname } from "next/navigation";
import { Home, Plus, Trophy, UserRound } from "lucide-react";
import { useAuth } from "./auth-provider";

const tabs = [
  { id: "home", href: "/", label: "ホーム", icon: Home },
  { id: "post", href: "/upload", label: "投稿", icon: Plus },
  { id: "competitions", href: "/competitions", label: "大会", icon: Trophy },
  { id: "me", href: "/me", label: "マイページ", icon: UserRound },
] as const;

function hidesTabBar(pathname: string) {
  if (pathname === "/login" || pathname === "/register" || pathname === "/reset-password") return true;
  if (pathname === "/me/edit") return true;
  if (pathname.startsWith("/posts/")) return true;
  return false;
}

function activeTab(pathname: string) {
  if (pathname === "/") return "home";
  if (pathname === "/upload") return "post";
  if (pathname === "/competitions" || pathname.startsWith("/competitions/")) return "competitions";
  if (pathname === "/me" || pathname === "/about") return "me";
  return "";
}

export function BottomNav() {
  const pathname = usePathname();
  const { session } = useAuth();
  if (hidesTabBar(pathname)) return null;

  const current = activeTab(pathname);

  return (
    <>
      <div className="h-[calc(3.5rem+env(safe-area-inset-bottom))] md:hidden" aria-hidden />
      <nav
        className="fixed inset-x-0 bottom-0 z-30 border-t border-zinc-200 bg-white md:hidden"
        style={{ paddingBottom: "env(safe-area-inset-bottom)" }}
        aria-label="メイン"
      >
        <ul className="mx-auto grid h-14 max-w-5xl grid-cols-4">
          {tabs.map((tab) => {
            const Icon = tab.icon;
            const active = current === tab.id;
            const href = tab.id === "post" && !session ? "/login" : tab.href;
            return (
              <li key={tab.id}>
                <Link
                  href={href}
                  className={`flex h-14 min-h-11 flex-col items-center justify-center gap-0.5 text-[11px] ${
                    active ? "font-semibold text-blue-700" : "text-zinc-500"
                  }`}
                  aria-current={active ? "page" : undefined}
                >
                  <Icon className="size-5" strokeWidth={active ? 2.4 : 2} aria-hidden />
                  {tab.label}
                </Link>
              </li>
            );
          })}
        </ul>
      </nav>
    </>
  );
}
