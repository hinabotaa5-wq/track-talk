"use client";

import { useAuth } from "@/components/auth-provider";
import { CoachAvailabilityEditor } from "@/components/coach-availability";
import { CoachSubpageShell } from "@/components/coach-subpage";
import { PageLoading } from "@/components/page-loading";
import { useRouter } from "next/navigation";
import { useEffect } from "react";

// 対応可能カレンダー。ランチャーのタイルから開く。
export default function CoachAvailabilityPage() {
  const { session, token, loading } = useAuth();
  const router = useRouter();

  useEffect(() => {
    if (!loading && !session) {
      router.replace(`/login?next=${encodeURIComponent("/coach/availability")}`);
    }
  }, [loading, session, router]);

  if (loading || !session) return <PageLoading screen label="読み込み中..." />;

  return (
    <CoachSubpageShell title="カレンダー">
      <CoachAvailabilityEditor token={token} />
    </CoachSubpageShell>
  );
}
