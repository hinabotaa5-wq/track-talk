"use client";
import { L } from "@/lib/i18n";

import { useAuth } from "@/components/auth-provider";
import { PageLoading } from "@/components/page-loading";
import { SupportConsultDetail } from "@/components/support-consult-detail";
import { apiFetch } from "@/lib/api";
import type { SupportConsult } from "@/lib/support-consult";
import Link from "next/link";
import { useParams, useRouter } from "next/navigation";
import { useEffect, useState } from "react";

// 無料相談の中身。回答の確認、コーチの聞き直しへの答え、提案の確認と申し込み。
export default function SupportConsultPage() {
  const { id } = useParams<{ id: string }>();
  const { session, token, loading } = useAuth();
  const router = useRouter();
  const [consult, setConsult] = useState<SupportConsult | null | undefined>(undefined);

  useEffect(() => {
    if (!loading && !session) router.replace(`/login?next=${encodeURIComponent(`/coaching/support-consults/${id}`)}`);
  }, [loading, session, router, id]);

  useEffect(() => {
    if (!token) return;
    let cancelled = false;
    apiFetch<{ consult: SupportConsult }>(`/v1/support-consults/${id}`, { token })
      .then((d) => { if (!cancelled) setConsult(d.consult); })
      .catch(() => { if (!cancelled) setConsult(null); });
    return () => { cancelled = true; };
  }, [token, id]);

  if (loading || !session || consult === undefined) return <PageLoading screen label={L("読み込み中...")} />;
  return (
    <div className="mx-auto max-w-md space-y-4">
      <h1 className="text-2xl font-bold">{L("無料相談")}</h1>
      {consult === null ? <p className="text-sm text-red-400">{L("相談を読み込めませんでした。")}</p> : (
        <SupportConsultDetail consult={consult} role="athlete" token={token} onChange={setConsult} />
      )}
      <Link href="/coaching/support-consults" className="block text-sm text-blue-300 underline">{L("相談の一覧へ")}</Link>
    </div>
  );
}
