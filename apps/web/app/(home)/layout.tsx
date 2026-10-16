import { Suspense } from "react";
import { CoachRecruitBanner } from "@/components/coach-recruit-banner";
import { HomeFilters } from "@/components/home-filters";

// The filters live in the layout so they stay mounted while loading.tsx replaces only the feed.
// The (home) route group keeps that loading.tsx from applying to every other route.
export default function HomeLayout({ children }: { children: React.ReactNode }) {
  return (
    <div className="-mx-4 -mt-6 md:mx-0 md:mt-0">
      <h1 className="sr-only">ホーム</h1>
      <Suspense>
        <HomeFilters />
      </Suspense>
      <div className="space-y-4 px-4 pb-4 md:px-0">
        <CoachRecruitBanner />
        {children}
      </div>
    </div>
  );
}
