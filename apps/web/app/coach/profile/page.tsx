"use client";

import { useAuth } from "@/components/auth-provider";
import { CoachPhotoForm } from "@/components/coach-photo";
import { CoachSubpageShell } from "@/components/coach-subpage";
import { PageLoading } from "@/components/page-loading";
import { apiFetch } from "@/lib/api";
import { coachApplyPath } from "@/lib/coach-recruit";
import type { CoachApplication } from "@/lib/types";
import Link from "next/link";
import { useRouter } from "next/navigation";
import { useEffect, useState } from "react";

// 顔写真と応募内容の確認。ランチャーのタイルから開く。
export default function CoachProfilePage() {
  const { session, profile, token, loading } = useAuth();
  const router = useRouter();
  const [coach, setCoach] = useState<CoachApplication | null | undefined>(undefined);

  useEffect(() => {
    if (!loading && !session) {
      router.replace(`/login?next=${encodeURIComponent("/coach/profile")}`);
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

  if (loading || !session || coach === undefined) return <PageLoading screen label="読み込み中..." />;

  return (
    <CoachSubpageShell title="プロフィール">
      {coach ? (
        <>
          <CoachPhotoForm
            application={coach}
            nickname={coach.name || profile?.nickname || null}
            token={token}
            onSaved={setCoach}
          />
          <Link href={coachApplyPath} className="block text-sm text-blue-300 underline">
            応募内容を確認する
          </Link>
        </>
      ) : (
        <p className="text-sm text-red-400">プロフィールを読み込めませんでした</p>
      )}
    </CoachSubpageShell>
  );
}
