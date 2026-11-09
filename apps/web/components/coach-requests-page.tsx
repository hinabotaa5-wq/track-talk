"use client";

import { useAuth } from "@/components/auth-provider";
import { CoachRequestsSection } from "@/components/coach-requests";
import { CoachSubpageShell } from "@/components/coach-subpage";
import { PageLoading } from "@/components/page-loading";
import { useRouter } from "next/navigation";
import { useEffect } from "react";

// 届いた依頼（offers）と添削中（reviewing）は別の画面。ランチャーの2タイルがそれぞれを開く。
const views = {
  offers: { path: "/coach/offers", title: "届いた依頼" },
  reviewing: { path: "/coach/reviewing", title: "添削中" },
} as const;

export function CoachRequestsPage({ view }: { view: keyof typeof views }) {
  const { session, token, loading } = useAuth();
  const router = useRouter();
  const { path, title } = views[view];

  useEffect(() => {
    if (!loading && !session) {
      router.replace(`/login?next=${encodeURIComponent(path)}`);
    }
  }, [loading, session, router, path]);

  if (loading || !session) return <PageLoading screen label="読み込み中..." />;

  return (
    <CoachSubpageShell title={title}>
      <CoachRequestsSection token={token} view={view} />
    </CoachSubpageShell>
  );
}
