import { Suspense } from "react";
import { HomeFilters } from "@/components/home-filters";
import { PostFab } from "@/components/post-fab";

// 絞り込みは layout に置き、loading.tsx がフィードだけを差し替えるあいだも表示したままにする。
// (home) のルートグループで、その loading.tsx がほかのすべてのルートにかからないようにする。
export default function HomeLayout({ children }: { children: React.ReactNode }) {
  return (
    <div className="-mx-4 -mt-6 -mb-6 md:mx-0 md:mt-0">
      <h1 className="sr-only">ホーム</h1>
      <div className="home-filter-anchor sticky z-30" style={{ top: "var(--home-header-height, calc(3.8125rem + env(safe-area-inset-top)))" }}>
        <Suspense>
          <HomeFilters />
        </Suspense>
      </div>
      <div className="space-y-4 px-4 md:px-0">{children}</div>
      <PostFab />
    </div>
  );
}
