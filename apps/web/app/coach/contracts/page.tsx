"use client";

import { useAuth } from "@/components/auth-provider";
import { CoachContractsSection } from "@/components/coaching-contracts";
import { CoachSubpageShell } from "@/components/coach-subpage";
import { PageLoading } from "@/components/page-loading";
import { useRouter } from "next/navigation";
import { useEffect } from "react";

// サポートプランの契約・申込み。ランチャーのタイルから開く。
export default function CoachContractsPage() {
  const { session, token, loading } = useAuth();
  const router = useRouter();

  useEffect(() => {
    if (!loading && !session) {
      router.replace(`/login?next=${encodeURIComponent("/coach/contracts")}`);
    }
  }, [loading, session, router]);

  if (loading || !session) return <PageLoading screen label="読み込み中..." />;

  return (
    <CoachSubpageShell title="契約・申込み">
      <CoachContractsSection token={token} />
    </CoachSubpageShell>
  );
}
