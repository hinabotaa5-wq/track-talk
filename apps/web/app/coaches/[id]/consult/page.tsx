"use client";

import { useAuth } from "@/components/auth-provider";
import { PageLoading } from "@/components/page-loading";
import { SupportConsultForm } from "@/components/support-consult-form";
import { apiFetch } from "@/lib/api";
import type { PublicSupportConsult } from "@/lib/support-consult";
import Link from "next/link";
import { useParams, useRouter } from "next/navigation";
import { useEffect, useState } from "react";

// 大会までプランの無料相談フォーム。
export default function CoachConsultPage() {
  const { id } = useParams<{ id: string }>();
  const { session, token, loading } = useAuth();
  const router = useRouter();
  const [consult, setConsult] = useState<PublicSupportConsult | null | undefined>(undefined);
  const [minor, setMinor] = useState<boolean | null>(null);

  useEffect(() => {
    if (!loading && !session) router.replace(`/login?next=${encodeURIComponent(`/coaches/${id}/consult`)}`);
  }, [loading, session, router, id]);

  useEffect(() => {
    let cancelled = false;
    apiFetch<{ consult: PublicSupportConsult | null }>(`/v1/coaches/${id}/support-consult`)
      .then((d) => { if (!cancelled) setConsult(d.consult); })
      .catch(() => { if (!cancelled) setConsult(null); });
    return () => { cancelled = true; };
  }, [id]);

  useEffect(() => {
    if (!token) return;
    let cancelled = false;
    apiFetch<{ is_minor?: boolean | null }>("/v1/me/coaching", { token })
      .then((d) => { if (!cancelled) setMinor(typeof d.is_minor === "boolean" ? d.is_minor : null); })
      .catch(() => {});
    return () => { cancelled = true; };
  }, [token]);

  if (loading || !session || consult === undefined) return <PageLoading screen label="読み込み中..." />;
  return (
    <div className="mx-auto max-w-md space-y-4">
      <h1 className="text-2xl font-bold">大会までプランの無料相談</h1>
      {consult === null ? (
        <p className="text-sm text-slate-200">このコーチはいま相談を受け付けていません。<Link href={`/coaches/${id}`} className="underline">コーチのページへ戻る</Link></p>
      ) : (
        <SupportConsultForm coachID={id} consult={consult} token={token} minor={minor} onSent={(c) => router.replace(`/coaching/support-consults/${c.id}`)} />
      )}
    </div>
  );
}
