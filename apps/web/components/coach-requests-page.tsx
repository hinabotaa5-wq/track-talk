"use client";

import { useAuth } from "@/components/auth-provider";
import { CoachRequestsSection } from "@/components/coach-requests";
import { CoachSubpageShell } from "@/components/coach-subpage";
import { PageLoading } from "@/components/page-loading";
import { parseLocale, t, type I18nKey } from "@/lib/i18n";
import { useRouter } from "next/navigation";
import { useEffect } from "react";

// 届いた依頼（offers）と添削中（reviewing）は別の画面。ランチャーの2タイルがそれぞれを開く。
const views = {
  offers: { path: "/coach/offers", title: "coach.offersTitle" },
  reviewing: { path: "/coach/reviewing", title: "coach.reviewingTitle" },
} as const satisfies Record<string, { path: string; title: I18nKey }>;

export function CoachRequestsPage({ view }: { view: keyof typeof views }) {
  const { session, token, profile, loading } = useAuth();
  const locale = parseLocale(profile?.locale);
  const router = useRouter();
  const { path, title } = views[view];

  useEffect(() => {
    if (!loading && !session) {
      router.replace(`/login?next=${encodeURIComponent(path)}`);
    }
  }, [loading, session, router, path]);

  if (loading || !session) return <PageLoading screen label={t(locale, "common.loading")} />;

  return (
    <CoachSubpageShell title={t(locale, title)}>
      <CoachRequestsSection token={token} view={view} locale={locale} />
    </CoachSubpageShell>
  );
}
