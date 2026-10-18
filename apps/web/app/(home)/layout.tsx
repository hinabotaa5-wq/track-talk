import { Suspense } from "react";
import { HomeFilters } from "@/components/home-filters";
import { PostFab } from "@/components/post-fab";

// 絞り込みは layout に置き、loading.tsx がフィードだけを差し替えるあいだも表示したままにする。
// (home) のルートグループで、その loading.tsx がほかのすべてのルートにかからないようにする。
export default function HomeLayout({ children }: { children: React.ReactNode }) {
  return (
    <div className="-mx-4 -mt-6 md:mx-0 md:mt-0">
      <h1 className="sr-only">ホーム</h1>
      <Suspense>
        <HomeFilters />
      </Suspense>
      <div className="space-y-4 px-4 pb-4 md:px-0">{children}</div>
      <PostFab />
    </div>
  );
}
