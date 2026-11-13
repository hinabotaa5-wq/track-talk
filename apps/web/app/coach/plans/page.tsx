"use client";
import { L } from "@/lib/i18n";

import { useAuth } from "@/components/auth-provider";
import { CoachPlansSection } from "@/components/coaching-plans";
import { CoachSubpageShell } from "@/components/coach-subpage";
import { PageLoading } from "@/components/page-loading";
import { apiFetch } from "@/lib/api";
import type { CoachApplication } from "@/lib/types";
import { useRouter } from "next/navigation";
import { useEffect, useState } from "react";

// サポートプラン（1ヶ月プランの受付設定）。ランチャーのタイルから開く。
export default function CoachPlansPage() {
  const { session, token, loading } = useAuth();
  const router = useRouter();
  const [coach, setCoach] = useState<CoachApplication | null | undefined>(undefined);

  useEffect(() => {
    if (!loading && !session) {
      router.replace(`/login?next=${encodeURIComponent("/coach/plans")}`);
    }
  }, [loading, session, router]);

  useEffect(() => {
    if (!token) return;
    let cancelled = false;
    apiFetch<{ application: CoachApplication | null }>("/v1/coach/me", { token })
      .then((d) => {
        if (!cancelled) setCoach(d.application);
      })
      .catch(() => {
        if (!cancelled) setCoach(null);
      });
    return () => {
      cancelled = true;
    };
  }, [token]);

  if (loading || !session || coach === undefined) return <PageLoading screen label={L("読み込み中...")} />;

  return (
    <CoachSubpageShell title={L("サポートプラン")}>
      {coach ? (
        <CoachPlansSection coach={coach} token={token} onCoachChange={setCoach} />
      ) : (
        <p className="text-sm text-red-400">{L("プランを読み込めませんでした")}</p>
      )}
    </CoachSubpageShell>
  );
}
