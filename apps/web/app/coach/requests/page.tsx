"use client";

import { useAuth } from "@/components/auth-provider";
import { CoachRequestsSection } from "@/components/coach-requests";
import { CoachSubpageShell } from "@/components/coach-subpage";
import { PageLoading } from "@/components/page-loading";
import { useRouter } from "next/navigation";
import { useEffect } from "react";

// 届いた依頼と添削中の一覧。ランチャーの2タイル（届いた依頼／添削中）がここを開く。
export default function CoachRequestsPage() {
  const { session, token, loading } = useAuth();
  const router = useRouter();

  useEffect(() => {
    if (!loading && !session) {
      router.replace(`/login?next=${encodeURIComponent("/coach/requests")}`);
    }
  }, [loading, session, router]);

  if (loading || !session) return <PageLoading screen label="読み込み中..." />;

  return (
    <CoachSubpageShell title="依頼">
      <p className="text-sm text-slate-300">
        新しい依頼が入ってから90分以内（23:00–07:00 はカウントが止まります）に引き受けないと、依頼は終了します。
      </p>
      <CoachRequestsSection token={token} />
    </CoachSubpageShell>
  );
}
