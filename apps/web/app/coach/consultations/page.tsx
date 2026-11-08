"use client";

import { useAuth } from "@/components/auth-provider";
import { CoachConsultationQueue } from "@/components/coaching-consultations";
import { CoachSubpageShell } from "@/components/coach-subpage";
import { PageLoading } from "@/components/page-loading";
import { useRouter } from "next/navigation";
import { useEffect } from "react";

// 追加相談のキュー。ランチャーのタイルから開く。
export default function CoachConsultationsPage() {
  const { session, token, loading } = useAuth();
  const router = useRouter();

  useEffect(() => {
    if (!loading && !session) {
      router.replace(`/login?next=${encodeURIComponent("/coach/consultations")}`);
    }
  }, [loading, session, router]);

  if (loading || !session) return <PageLoading screen label="読み込み中..." />;

  return (
    <CoachSubpageShell title="追加相談">
      <CoachConsultationQueue token={token} />
    </CoachSubpageShell>
  );
}
