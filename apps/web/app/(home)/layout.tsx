"use client";

import { L } from "@/lib/i18n";
import { Suspense } from "react";
import { HomeFilters } from "@/components/home-filters";

// 絞り込みは layout に置き、loading.tsx がフィードだけを差し替えるあいだも表示したままにする。
// (home) のルートグループで、その loading.tsx がほかのすべてのルートにかからないようにする。
export default function HomeLayout({ children }: { children: React.ReactNode }) {
  return (
    <div className="home-layout -mx-4 -mt-6 -mb-6 md:mt-0">
      <h1 className="sr-only">{L("ホーム")}</h1>
      <div className="home-filter-anchor sticky z-30 mx-auto max-w-5xl md:px-4" style={{ top: "var(--home-header-height, calc(3.8125rem + env(safe-area-inset-top)))" }}>
        <Suspense>
          <HomeFilters />
        </Suspense>
      </div>
      <div className="home-feed-content space-y-4">{children}</div>
    </div>
  );
}
