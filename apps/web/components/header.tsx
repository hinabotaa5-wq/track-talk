"use client";

import Link from "next/link";
import { MainTabLink } from "./main-tab-link";
import { appIconColors, appIconLaneWidth, appIconLanes } from "./app-icon-mark";
import { ChevronLeft } from "lucide-react";
import { useAuth } from "./auth-provider";
import { HeaderAccountPanel } from "./header-account-panel";
import { FeedbackBox } from "./feedback-box";
import { MenuContestBar } from "./menu-contest-bar";
import { NotificationBell } from "./notification-bell";
import { TodayMeetBalloon } from "./today-meet-balloon";
import { useCurrentMainTab, headerBackFallback, hasBackOnlyHeader, hidesHeader, mainTabHref, mainTabs, showsBackButton, showsHeaderAccountActions } from "./bottom-nav";
import { useGuardedLinkClick, useLeaveGuard } from "./leave-guard";
import { profileIncomplete } from "@/lib/profile-gate";
import { usePathname, useRouter } from "next/navigation";
import { useSyncExternalStore } from "react";

// useSearchParams はレイアウトで使うと Suspense が要り、全ページが動的になるので、ここでは location から読む。
// パスが変わるたびに Header は描き直されるので、その時点のクエリが取れる。
const noSubscribe = () => () => {};
function useLocationSearch() {
  return useSyncExternalStore(noSubscribe, () => window.location.search, () => "");
}

// アプリアイコン(components/app-icon-mark.tsx)の背景を除いた3レーンのマーク。
// アイコンでは半透明の白に空色が透けて内側ほど水色に見えるが、濃紺の上では灰色になるので、
// 透けた後の見え方の色を直接塗る(外側→内側)。
const headerLaneColors = ["#ffffff", "#c4ecff", appIconColors.from] as const;

// 512×512 の座標のうちレーンのある範囲(線の太さを含む)だけを切り出して表示する。
function TrackMark() {
  return (
    <svg viewBox="115 125 280 270" className="h-7 w-[29px] shrink-0" aria-hidden="true">
      <g fill="none" strokeWidth={appIconLaneWidth} strokeLinecap="round">
        {appIconLanes.map((lane, i) => (
          <path key={lane.d} d={lane.d} stroke={headerLaneColors[i]} />
        ))}
      </g>
    </svg>
  );
}

function TrackLogo() {
  return (
    <span className="flex items-center gap-2.5 font-black text-white text-xl">
      <TrackMark />
      <span>
        Track<span className="font-bold" style={{ color: appIconColors.from }}>Talk</span>
      </span>
    </span>
  );
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

  function goBack() {
    const go = () => {
      if (pathname === "/coach/apply") router.replace("/coach");
      else if (pathname === "/coach/about") router.replace("/coach/apply");
      else if (window.history.length > 1) router.back();
      else router.replace(headerBackFallback(pathname));
    };
    if (!confirmLeave(go)) go();
  }

  const current = useCurrentMainTab(pathname);

  // ヘッダーを出さない画面でも、iPhone の上の切り欠きに中身が重ならないよう余白だけ残す。
  if (hidesHeader(pathname, search, profileIncomplete(profile))) return <div aria-hidden style={{ paddingTop: "env(safe-area-inset-top)" }} />;

  return (
    // 1 割でも透けると、下に流れた文字が blur でにじんでヘッダーがぼやけて見える。
    // blur は使わず、背景と同じ色で完全にふさぐ。
    <header
      className="header-overscroll-fill sticky top-0 z-40 border-b border-white/15 bg-[#16243a]"
      style={{ paddingTop: "env(safe-area-inset-top)" }}
    >
      <div className="mx-auto flex max-w-5xl items-center justify-between gap-4 px-4 py-2.5">
        <div className="flex min-w-0 items-center gap-4">
          {showBack ? (
            <button
              type="button"
              onClick={goBack}
              aria-label="戻る"
              className="-ml-2 inline-flex size-10 shrink-0 items-center justify-center rounded-full text-slate-300 hover:bg-white/16 transition active:scale-[0.98]"
            >
              <ChevronLeft className="size-6" strokeWidth={2.4} aria-hidden />
            </button>
          ) : (
            <Link prefetch={false} href="/" onClick={(event) => guardedClick(event, "/")} aria-label="TrackTalk" className="group flex items-center transition active:scale-[0.98]">
              <TrackLogo />
            </Link>
          )}
        </div>
        {backOnly ? null : (
          <div className="flex shrink-0 items-center gap-1">
            <nav className="hidden items-center gap-1 md:flex" aria-label="メイン">
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
                        ? "border border-white/35 bg-white/20 font-semibold text-white"
                        : "text-slate-200 hover:bg-white/12 hover:text-white"
                    }`}
                  >
                    <Icon className="size-4" strokeWidth={active ? 2.4 : 2} aria-hidden />
                    <span>{tab.label}</span>
                  </MainTabLink>
                );
              })}
            </nav>
            {showHeaderAccountActions ? <TodayMeetBalloon /> : null}
            {session && showHeaderAccountActions ? (
              <>
                <FeedbackBox />
                <NotificationBell />
              </>
            ) : null}
            {showHeaderAccountActions ? (
              <HeaderAccountPanel key={`me:${pathname}`} signedIn={!!session} />
            ) : null}
          </div>
        )}
      </div>
      {/* エグ練大会の開催中だけ、ホームのヘッダーに締め切りまでの残り時間を出す。 */}
      {pathname === "/" ? <MenuContestBar /> : null}
    </header>
  );
}
