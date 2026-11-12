"use client";

import { useAuth } from "@/components/auth-provider";
import { CoachRequestsSection } from "@/components/coach-requests";
import { CoachSubpageShell } from "@/components/coach-subpage";
import { PageLoading } from "@/components/page-loading";
import { t } from "@/lib/i18n";
import { useLocale } from "@/components/locale-provider";
import { useRouter } from "next/navigation";
import { useEffect } from "react";

export function CoachRequestsPage() {
  const { session, token, loading } = useAuth();
  const locale = useLocale();
  const router = useRouter();
  const path = "/coach/reviewing";

  useEffect(() => {
    if (!loading && !session) {
      router.replace(`/login?next=${encodeURIComponent(path)}`);
    }
  }, [loading, session, router, path]);

  if (loading || !session) return <PageLoading screen label={t(locale, "common.loading")} />;

  return (
    <CoachSubpageShell title={t(locale, "coach.reviewingTitle")}>
      <CoachRequestsSection token={token} locale={locale} />
    </CoachSubpageShell>
  );
}
