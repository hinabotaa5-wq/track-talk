"use client";
import { L } from "@/lib/i18n";

import { useAuth } from "@/components/auth-provider";
import { PageLoading } from "@/components/page-loading";
import { apiFetch } from "@/lib/api";
import { formatDateTime } from "@/lib/format";
import { supportConsultStatusLabels, type SupportConsult } from "@/lib/support-consult";
import Link from "next/link";
import { useRouter } from "next/navigation";
import { useEffect, useState } from "react";

// 自分が送った大会までプランの無料相談。
export default function MySupportConsultsPage() {
  const { session, token, loading } = useAuth();
  const router = useRouter();
  const [rows, setRows] = useState<SupportConsult[] | null>(null);
  const [error, setError] = useState("");

  useEffect(() => {
    if (!loading && !session) router.replace(`/login?next=${encodeURIComponent("/coaching/support-consults")}`);
  }, [loading, session, router]);

  useEffect(() => {
    if (!token) return;
    let cancelled = false;
    apiFetch<{ consults: SupportConsult[] }>("/v1/me/support-consults", { token })
      .then((d) => { if (!cancelled) setRows(d.consults); })
      .catch(() => { if (!cancelled) setError(L("無料相談を読み込めませんでした")); });
    return () => { cancelled = true; };
  }, [token]);

  if (loading || !session || (rows === null && !error)) return <PageLoading screen label={L("読み込み中...")} />;
  return (
    <div className="mx-auto max-w-md space-y-4">
      <h1 className="text-2xl font-bold">{L("大会までプランの無料相談")}</h1>
      {error ? <p role="alert" className="text-sm text-red-400">{error}</p> : null}
      {rows?.length === 0 ? <p className="text-sm text-slate-200">{L("まだ相談はありません。コーチのページから、無料で相談できます。")}</p> : null}
      <ul className="space-y-2">
        {rows?.map((c) => (
          <li key={c.id}>
            <Link href={`/coaching/support-consults/${c.id}`} className="block rounded-xl border border-white/20 bg-white/10 p-3 text-sm hover:bg-white/15">
              <span className="block font-semibold">{c.coach_nickname || L("コーチ")}</span>
              <span className="block text-slate-200">{supportConsultStatusLabels[c.status]}</span>
              <span className="block text-xs text-slate-300">{formatDateTime(c.created_at)}</span>
            </Link>
          </li>
        ))}
      </ul>
    </div>
  );
}
