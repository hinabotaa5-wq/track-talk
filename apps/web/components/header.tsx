"use client";

import Link from "next/link";
import { MainTabLink } from "./main-tab-link";
import { ChevronLeft } from "lucide-react";
import { useAuth } from "./auth-provider";
import { HeaderAccountPanel } from "./header-account-panel";
import { AppLogo } from "./app-icon-mark";
import { FeedbackBox } from "./feedback-box";
import { InstagramLink } from "./instagram-link";
import { MenuContestBar } from "./menu-contest-bar";
import { NotificationBell } from "./notification-bell";
import { TodayMeetBalloon } from "./today-meet-balloon";
import { useCurrentMainTab, headerBackFallback, isCoachChildPage, hasBackOnlyHeader, hidesHeader, mainTabHref, mainTabs, showsBackButton, showsHeaderAccountActions } from "./bottom-nav";
import { useGuardedLinkClick, useLeaveGuard } from "./leave-guard";
import { trackHeaderHistory } from "@/lib/header-history";
import { mainTabLabel, t } from "@/lib/i18n";
import { useLocale } from "@/components/locale-provider";
import { profileIncomplete } from "@/lib/profile-gate";
import { usePathname, useRouter } from "next/navigation";
import { useEffect, useRef, useSyncExternalStore } from "react";

// useSearchParams はレイアウトで使うと Suspense が要り、全ページが動的になるので、ここでは location から読む。
// パスが変わるたびに Header は描き直されるので、その時点のクエリが取れる。
const noSubscribe = () => () => {};
function useLocationSearch() {
  return useSyncExternalStore(noSubscribe, () => window.location.search, () => "");
}

export function Header() {
  const { session, profile } = useAuth();
  const router = useRouter();
  const pathname = usePathname();
  const search = useLocationSearch();
  const showBack = showsBackButton(pathname, search);
  const showHeaderAccountActions = showsHeaderAccountActions(pathname, search);
  const backOnly = hasBackOnlyHeader(pathname, search);
  const { confirmLeave } = useLeaveGuard();
  const guardedClick = useGuardedLinkClick();

  const history = useRef<ReturnType<typeof trackHeaderHistory> | null>(null);
  useEffect(() => {
    let active = true;
    let tracker: ReturnType<typeof trackHeaderHistory> | undefined;
    // 親の Next.js Router が History API を準備したあとに追跡を始める。
    // Strict Mode の破棄された初回エフェクトでは登録しない。
    queueMicrotask(() => {
      if (!active) return;
      tracker = trackHeaderHistory(window.history, window.location);
      history.current = tracker;
    });
    return () => {
      active = false;
      tracker?.dispose();
      history.current = null;
    };
  }, []);

  function goBack() {
    const go = () => {
      if (pathname === "/coach/apply") router.replace("/coach");
      else if (pathname === "/coach/about") router.replace("/coach/apply");
      else if (isCoachChildPage(pathname)) router.replace("/coach");
      else if (history.current?.canGoBack(!!session)) router.back();
      else router.replace(headerBackFallback(pathname));
    };
    if (!confirmLeave(go)) go();
  }

  const current = useCurrentMainTab(pathname);
  const locale = useLocale();

  // ヘッダーを出さない画面でも、iPhone の上の切り欠きに中身が重ならないよう余白だけ残す。
  if (hidesHeader(pathname, search, profileIncomplete(profile))) return <div aria-hidden style={{ paddingTop: "env(safe-area-inset-top)" }} />;

  return (
    // 1 割でも透けると、下に流れた文字が blur でにじんでヘッダーがぼやけて見える。
    // blur は使わず、背景と同じ色で完全にふさぐ。
    <header
      className="sticky top-0 z-40 border-b border-white/15 bg-background"
      style={{ paddingTop: "env(safe-area-inset-top)" }}
    >
      <div className="mx-auto flex max-w-5xl items-center justify-between gap-4 px-4 py-2.5">
        <div className="flex min-w-0 items-center gap-4">
          {showBack ? (
            <button
              type="button"
              onClick={goBack}
              aria-label={t(locale, "common.back")}
              className="-ml-2 inline-flex size-10 shrink-0 items-center justify-center rounded-full text-slate-300 hover:bg-white/16 transition active:scale-[0.98]"
            >
              <ChevronLeft className="size-6" strokeWidth={2.4} aria-hidden />
            </button>
          ) : (
            <Link prefetch={false} href="/" onClick={(event) => guardedClick(event, "/")} aria-label="TrackTalk" className="group flex items-center transition active:scale-[0.98]">
              <AppLogo />
            </Link>
          )}
        </div>
        {backOnly ? null : (
          <div className="flex shrink-0 items-center gap-1">
            <nav className="hidden items-center gap-1 md:flex" aria-label={t(locale, "nav.main")} data-main-nav="">
              {mainTabs.map((tab) => {
                const Icon = tab.icon;
                const active = current === tab.id;
                return (
                  <MainTabLink
                    key={tab.id}
                    href={mainTabHref(tab.id, !!session)}
                    onClick={(event) => guardedClick(event, mainTabHref(tab.id, !!session))}
                    aria-current={active ? "page" : undefined}
                    className={`inline-flex items-center gap-1.5 rounded-full px-3 py-2 text-sm transition active:scale-[0.98] ${
                      active
                        ? "bg-blue-500/15 font-semibold text-blue-300"
                        : "text-slate-200 hover:bg-blue-500/10 hover:text-blue-300"
                    }`}
                  >
                    <Icon className="size-4" strokeWidth={active ? 2.4 : 2} aria-hidden />
                    <span>{mainTabLabel(tab.id, locale)}</span>
                  </MainTabLink>
                );
              })}
            </nav>
            {showHeaderAccountActions ? <TodayMeetBalloon /> : null}
            {showHeaderAccountActions ? <InstagramLink /> : null}
            {session && showHeaderAccountActions ? (
              <>
                <FeedbackBox />
                <NotificationBell />
              </>
            ) : null}
            {showHeaderAccountActions ? (
              <HeaderAccountPanel key={`me:${pathname}`} signedIn={!!session} label={mainTabLabel("me", locale)} />
            ) : null}
          </div>
        )}
      </div>
      {/* エグ練グランプリの開催中だけ、ホームのヘッダーに締め切りまでの残り時間を出す。 */}
      {pathname === "/" ? <MenuContestBar /> : null}
    </header>
  );
}
