"use client";

import Link from "next/link";
import { usePathname } from "next/navigation";
import { GraduationCap, HeartPulse, Home, Trophy, UserRound } from "lucide-react";
import { useGuardedLinkClick } from "./leave-guard";
import { useAuth } from "./auth-provider";

export const mainTabs = [
  { id: "home", href: "/", label: "ホーム", icon: Home },
  { id: "competitions", href: "/competitions", label: "大会", icon: Trophy },
  { id: "coaching", href: "/coaching", label: "コーチ", icon: GraduationCap },
  { id: "injury", href: "/injury", label: "ケガ", icon: HeartPulse },
  { id: "me", href: "/me", label: "マイページ", icon: UserRound },
] as const;

export function mainTabHref(id: (typeof mainTabs)[number]["id"], signedIn: boolean) {
  const tab = mainTabs.find((item) => item.id === id);
  if (!tab) return "/";
  if (id !== "home" && id !== "coaching" && !signedIn) return "/login";
  return tab.href;
}

export function activeMainTab(pathname: string) {
  if (pathname === "/" || pathname.startsWith("/reviews/")) return "home";
  if (pathname === "/coaching") return "coaching";
  if (pathname === "/competitions" || pathname.startsWith("/competitions/")) return "competitions";
  if (pathname === "/injury" || pathname.startsWith("/injury/")) return "injury";
  if (pathname === "/me") return "me";
  return "";
}

export function showsBackButton(pathname: string) {
  if (pathname === "/register" || pathname === "/reset-password") return true;
  if (pathname === "/upload") return true;
  if (pathname === "/me/contact" || pathname === "/me/delete" || pathname === "/me/profile" || pathname === "/me/injury" || pathname === "/me/saved") return true;
  if (pathname === "/guidelines" || pathname === "/terms" || pathname === "/privacy") return true;
  if (pathname === "/competitions/major-hosts") return true;
  if (pathname === "/competitions/regional") return true;
  if (pathname === "/menu-contest") return true;
  if (/^\/injury\/[^/]+\/new$/.test(pathname)) return true;
  if (pathname.startsWith("/posts/")) return true;
  if (pathname.startsWith("/reviews/")) return true;
  return false;
}

// 投稿はホームの「＋」から開く詳細画面なので、そこから先の作成画面でも下のメニューは出さない。
const bottomNavHidden = new Set([
  "/login",
  "/register",
  "/reset-password",
  "/upload",
  "/posts/new",
  "/posts/url",
  "/posts/video",
  "/posts/recruit",
  "/posts/practice",
]);

export function hidesBottomNav(pathname: string) {
  return bottomNavHidden.has(pathname);
}

export function BottomNav() {
  const pathname = usePathname();
  const { session } = useAuth();
  const guardedClick = useGuardedLinkClick();

  if (hidesBottomNav(pathname)) return null;

  const current = activeMainTab(pathname);

  return (
    <>
      <div className="h-[calc(3.5rem+env(safe-area-inset-bottom))] md:hidden [body[data-comment-typing]_&]:hidden" aria-hidden />
      <nav
        className="fixed inset-x-0 bottom-0 z-30 border-t border-white/20 glass bg-white/8 backdrop-blur-md md:hidden [body[data-comment-typing]_&]:hidden"
        style={{ paddingBottom: "env(safe-area-inset-bottom)" }}
        aria-label="メイン"
      >
        <ul className="mx-auto grid h-14 max-w-5xl grid-cols-5">
          {mainTabs.map((tab) => {
            const Icon = tab.icon;
            const active = current === tab.id;
            const href = mainTabHref(tab.id, !!session);
            return (
              <li key={tab.id}>
                <Link
                  href={href}
                  onClick={(event) => guardedClick(event, href)}
                  className={`flex h-14 min-h-11 flex-col items-center justify-center gap-0.5 text-[11px] transition-transform duration-150 active:scale-95 ${
                    active ? "font-semibold text-blue-400" : "text-slate-200 hover:text-white"
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
