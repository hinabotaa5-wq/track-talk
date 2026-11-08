"use client";

import { useAuth } from "@/components/auth-provider";
import { CoachEarningsSection } from "@/components/coach-payouts";
import { CoachSubpageShell } from "@/components/coach-subpage";
import { PageLoading } from "@/components/page-loading";
import { useRouter } from "next/navigation";
import { useEffect } from "react";

// 報酬と受け取り口座。ランチャーのタイルから開く。
export default function CoachPayoutsPage() {
  const { session, token, loading } = useAuth();
  const router = useRouter();

  useEffect(() => {
    if (!loading && !session) {
      router.replace(`/login?next=${encodeURIComponent("/coach/payouts")}`);
    }
  }, [loading, session, router]);

  if (loading || !session) return <PageLoading screen label="読み込み中..." />;

  return (
    <CoachSubpageShell title="報酬・口座">
      <CoachEarningsSection token={token} onReady={() => {}} />
    </CoachSubpageShell>
  );
}
