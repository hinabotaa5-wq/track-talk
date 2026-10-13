"use client";

import Link from "next/link";
import { ChevronLeft } from "lucide-react";
import { useAuth } from "./auth-provider";
import { NotificationBell } from "./notification-bell";
import { activeMainTab, hidesTabBar, mainTabHref, mainTabs, useRememberedCompetitionPrefecture } from "./bottom-nav";
import { usePathname, useRouter } from "next/navigation";

export function Header() {
  const { session } = useAuth();
  const router = useRouter();
  const pathname = usePathname();
  const competitionPrefecture = useRememberedCompetitionPrefecture();
  const showBack = hidesTabBar(pathname);

  function goBack() {
    if (window.history.length > 1) router.back();
    else router.push("/");
  }

  const current = activeMainTab(pathname);

  return (
    <header className={`border-b border-border bg-surface${showBack ? " sticky top-0 z-40" : ""}`}>
      <div className="mx-auto flex max-w-5xl items-center justify-between gap-4 px-4 py-3">
        <div className="flex min-w-0 items-center gap-4">
          {showBack ? (
            <button
              type="button"
              onClick={goBack}
              aria-label="戻る"
              className="-ml-2 inline-flex size-11 shrink-0 items-center justify-center rounded-md text-accent"
            >
              <ChevronLeft className="size-6" strokeWidth={2.4} aria-hidden />
            </button>
          ) : (
            <Link href="/" className="truncate text-lg font-bold tracking-tight text-accent">
              TRACK TALK
            </Link>
          )}
        </div>
        <div className="flex shrink-0 items-center gap-1">
          <nav className="hidden items-center gap-1 md:flex" aria-label="メイン">
            {mainTabs.map((tab) => {
              const Icon = tab.icon;
              const active = current === tab.id;
              return (
                <Link
                  key={tab.id}
                  href={mainTabHref(tab.id, !!session, competitionPrefecture)}
                  aria-current={active ? "page" : undefined}
                  className={`inline-flex items-center gap-1.5 rounded-full px-3 py-2 text-sm ${
                    active ? "bg-accent-soft font-semibold text-accent" : "text-muted hover:bg-surface-raised"
                  }`}
                >
                  <Icon className="size-4" strokeWidth={active ? 2.4 : 2} aria-hidden />
                  {tab.label}
                </Link>
              );
            })}
          </nav>
          {session && !showBack ? <NotificationBell /> : null}
        </div>
      </div>
    </header>
  );
}
