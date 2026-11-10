"use client";
import { L } from "@/lib/i18n";

import { useAuth } from "@/components/auth-provider";
import { CoachCommerceSection } from "@/components/coach-commerce";
import { CoachSubpageShell } from "@/components/coach-subpage";
import { PageLoading } from "@/components/page-loading";
import { useRouter } from "next/navigation";
import { useEffect } from "react";

// 特商法・身元の登録。ランチャーのタイルから開く。
export default function CoachCommercePage() {
  const { session, loading } = useAuth();
  const router = useRouter();

  useEffect(() => {
    if (!loading && !session) {
      router.replace(`/login?next=${encodeURIComponent("/coach/commerce")}`);
    }
  }, [loading, session, router]);

  if (loading || !session) return <PageLoading screen label={L("読み込み中...")} />;

  return (
    <CoachSubpageShell title={L("特商法・身元")}>
      <CoachCommerceSection />
    </CoachSubpageShell>
  );
}
