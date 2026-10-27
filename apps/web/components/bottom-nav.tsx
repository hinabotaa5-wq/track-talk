"use client";

import { MainTabLink } from "./main-tab-link";
import { useSyncExternalStore } from "react";
import { readAccountOrigin } from "@/lib/account-origin";
import { lastTabChangedEvent, readLastTab } from "@/lib/last-tab";
import { usePathname } from "next/navigation";
import { createLucideIcon, GraduationCap, HeartPulse, Home, Trophy, UserRound } from "lucide-react";
import { useGuardedLinkClick } from "./leave-guard";
import { useAuth } from "./auth-provider";
import { isInjuryBodyPart } from "@/lib/injury";

// メモのアイコン。lucide の NotebookPen から、左の4本線(ノートのリング)を外した形。
const MemoIcon = createLucideIcon("memo", [
  ["path", { d: "M13.4 2H6a2 2 0 0 0-2 2v16a2 2 0 0 0 2 2h12a2 2 0 0 0 2-2v-7.4", key: "body" }],
  [
    "path",
    {
      d: "M21.378 5.626a1 1 0 1 0-3.004-3.004l-5.01 5.012a2 2 0 0 0-.506.854l-.837 2.87a.5.5 0 0 0 .62.62l2.87-.837a2 2 0 0 0 .854-.506z",
      key: "pen",
    },
  ],
]);

export const mainTabs = [
  { id: "home", href: "/", label: "ホーム", icon: Home },
  { id: "competitions", href: "/competitions", label: "大会", icon: Trophy },
  { id: "coaching", href: "/coaching", label: "コーチ", icon: GraduationCap },
  { id: "memo", href: "/memo", label: "メモ", icon: MemoIcon },
  { id: "injury", href: "/injury", label: "ケガ", icon: HeartPulse },
] as const;

// マイページは下のメニューではなく、ヘッダー右上のアイコンから開く。
const meTab = { id: "me", href: "/me", label: "マイページ", icon: UserRound } as const;

export function mainTabHref(id: (typeof mainTabs)[number]["id"] | "me", signedIn: boolean) {
  const tab = id === "me" ? meTab : mainTabs.find((item) => item.id === id);
  if (!tab) return "/";
  // ログインが要るのはマイページだけ。メモはログインなしでも見られ、書こうとしたときにログインへ案内する。
  if (id === "me" && !signedIn) return "/login";
  return tab.href;
}

export function activeMainTab(pathname: string) {
  if (pathname === "/" || pathname.startsWith("/reviews/")) return "home";
  if (["/coaching", "/coaches", "/coach", "/guardian", "/me/coaching"].some((base) => pathname === base || pathname.startsWith(`${base}/`))) return "coaching";
  if (pathname === "/competitions" || pathname.startsWith("/competitions/")) return "competitions";
  if (pathname.startsWith("/prefectures/")) return "competitions";
  if (pathname === "/injury" || pathname.startsWith("/injury/") || pathname === "/me/injury") return "injury";
  if (pathname === "/memo" || pathname.startsWith("/memo/") || pathname === "/me/sense") return "memo";
  if (pathname === "/menu-contest") return "home";
  if (pathname === "/me") return "me";
  return "";
}

const noSubscribe = () => () => {};
function subscribeLastTab(callback: () => void) {
  window.addEventListener(lastTabChangedEvent, callback);
  window.addEventListener("storage", callback);
  return () => { window.removeEventListener(lastTabChangedEvent, callback); window.removeEventListener("storage", callback); };
}
const lastTabSnapshot = () => readLastTab() ?? "/";
export function useCurrentMainTab(pathname: string) {
  const origin = useSyncExternalStore(noSubscribe, () => pathname === "/login" ? new URLSearchParams(window.location.search).get("next") ?? readAccountOrigin() : readAccountOrigin(), () => "/");
  const last = useSyncExternalStore(subscribeLastTab, lastTabSnapshot, () => "/");
  const own = activeMainTab(pathname);
  if (own && own !== "me") return own;
  if (pathname === "/login" || pathname === "/me" || pathname.startsWith("/me/") || pathname.startsWith("/admin")) {
    return activeMainTab(origin.split("?")[0]) || activeMainTab(last.split("?")[0]) || "home";
  }
  // 投稿などURLだけで所属を判断できない詳細は、最後に開いたタブの色を保つ。
  return activeMainTab(last.split("?")[0]) || "home";
}

const injuryGroupPathPattern = /^\/injury\/([^/]+)$/;

// ケガのグループの画面(/injury/hamstring-strain など)。「データ」の /injury/data は含まない。
function isInjuryGroupPath(pathname: string) {
  const code = injuryGroupPathPattern.exec(pathname)?.[1];
  return !!code && isInjuryBodyPart(code);
}

// マイページの中の画面(/me/profile など)。マイページ本体の /me は含まない。
function isMeSubPage(pathname: string) {
  return pathname.startsWith("/me/");
}

// タブから開いた先の画面は共通ヘッダーで戻る。
function isDetailPage(pathname: string) {
  return pathname === "/coaches" || pathname === "/coach" || pathname === "/admin" ||
    ["/coaches/", "/coaching/", "/coach/", "/competitions/", "/prefectures/", "/guardian/", "/admin/"].some((prefix) => pathname.startsWith(prefix));
}

export function headerBackFallback(pathname: string) {
  if (pathname === "/admin") return "/me";
  if (pathname.startsWith("/admin/")) return "/admin";
  if (/^\/coaching\/personal\/plans\/[^/]+$/.test(pathname) || pathname.startsWith("/coaches/")) return "/coaches";
  if (pathname === "/coach/about") return "/coach/apply";
  if (pathname.startsWith("/coach/requests/")) return "/coach";
  if (pathname === "/coach" || pathname.startsWith("/coach/") || pathname.startsWith("/coaching/") || pathname === "/coaches") return "/coaching";
  if (pathname.startsWith("/competitions/")) return "/competitions";
  if (pathname.startsWith("/me/")) return "/me";
  return "/";
}

// ヘッダーを「戻る」ボタンだけにする画面。ロゴ・大会などのメニュー・通知は出さない。
// 下のメニューの「マイページ」から開いたログイン画面(next なし)は、ふつうのヘッダーのまま。
export function hasBackOnlyHeader(pathname: string, search = "") {
  if (isDetailPage(pathname)) return true;
  if (pathname === "/register" || pathname === "/reset-password" || pathname === "/reset-password/update") return true;
  if (pathname === "/login") return new URLSearchParams(search).has("next");
  if (isMeSubPage(pathname)) return true;
  return isInjuryGroupPath(pathname);
}

// ヘッダーを何も出さない画面。はじめにニックネームと種目を決める画面と、新しいパスワードを決める画面
// (リンクの有効期限切れの案内も含む)は、ほかへ移らずにその場で済ませてもらう。
// mustSetProfile は、まだニックネームか種目を決めていない人のとき true。
export function hidesHeader(pathname: string, search = "", mustSetProfile = false) {
  if (pathname === "/reset-password/update") return true;
  return pathname === "/me/profile" && (mustSetProfile || new URLSearchParams(search).has("onboarding"));
}

// search は "?next=/upload" のような URL のクエリ部分。
export function showsBackButton(pathname: string, search = "") {
  if (isDetailPage(pathname)) return true;
  if (pathname === "/register" || pathname === "/reset-password" || pathname === "/reset-password/update") return true;
  // 投稿の＋などログインが要るページから送られてきたときは、元の画面に戻れるようにする。
  // 下のメニューのタブから開いたときは next が無いので、ロゴのままにする。
  if (pathname === "/login") return new URLSearchParams(search).has("next");
  if (pathname === "/upload") return true;
  if (isMeSubPage(pathname)) return true;
  if (pathname === "/guide" || pathname === "/guidelines" || pathname === "/terms" || pathname === "/privacy" || pathname === "/contact") return true;
  if (pathname === "/competitions/major-hosts") return true;
  if (pathname === "/competitions/regional") return true;
  if (pathname === "/menu-contest") return true;
  if (/^\/injury\/[^/]+\/(new|stage)$/.test(pathname)) return true;
  if (isInjuryGroupPath(pathname)) return true;
  if (pathname.startsWith("/posts/")) return true;
  if (pathname.startsWith("/reviews/")) return true;
  return false;
}

// 通知・マイページ・管理アイコンを戻るボタンと同じ画面条件で表示する。
export function showsHeaderAccountActions(pathname: string, search = "") {
  return !showsBackButton(pathname, search) || pathname === "/me" || isMeSubPage(pathname);
}

// 投稿はホームの「＋」から開く詳細画面なので、そこから先の作成画面でも下のメニューは出さない。
// ログイン画面では出す(ログインしていなければ、ホームとコーチ以外のタブはログイン画面に戻る)。
const bottomNavHidden = new Set([
  "/register",
  "/reset-password",
  "/reset-password/update",
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
  const current = useCurrentMainTab(pathname);
  if (hidesBottomNav(pathname)) return null;

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
                <MainTabLink
                  pendingDelay={200}
                  href={href}
                  onClick={(event) => guardedClick(event, href)}
                  className={`flex h-14 min-h-11 flex-col items-center justify-center gap-0.5 text-xs transition-transform duration-150 active:scale-[0.98] ${
                    active ? "font-semibold text-blue-400" : "text-slate-200 hover:text-white"
                  }`}
                  aria-current={active ? "page" : undefined}
                >
                  <Icon className="size-5" strokeWidth={active ? 2.4 : 2} aria-hidden />
                  <span>{tab.label}</span>
                </MainTabLink>
              </li>
            );
          })}
        </ul>
      </nav>
    </>
  );
}
