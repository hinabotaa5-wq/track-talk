"use client";

import { useAuth } from "@/components/auth-provider";
import { apiFetch } from "@/lib/api";
import { contractStatusLabel } from "@/lib/coaching-plan";
import { formatDateTime } from "@/lib/format";
import type { CoachingContract } from "@/lib/types";
import Link from "next/link";
import { useRouter } from "next/navigation";
import { useEffect, useState } from "react";

// 自分が申し込んだパーソナル指導の一覧。
export default function MyCoachingContractsPage() {
  const { session, token, loading } = useAuth();
  const router = useRouter();
  const [contracts, setContracts] = useState<CoachingContract[] | null>(null);
  const [error, setError] = useState("");

  useEffect(() => {
    if (!loading && !session) {
      router.replace(`/login?next=${encodeURIComponent(window.location.pathname)}`);
    }
  }, [loading, session, router]);

  useEffect(() => {
    if (!token) return;
    let cancelled = false;
    apiFetch<{ contracts: CoachingContract[] }>("/v1/me/coaching/contracts", { token })
      .then((d) => {
        if (!cancelled) setContracts(d.contracts);
      })
      .catch(() => {
        if (!cancelled) setError("読み込みに失敗しました");
      });
    return () => {
      cancelled = true;
    };
  }, [token]);

  if (error) return <p className="text-sm text-red-400">{error}</p>;
  if (loading || !session || !contracts) return <p className="text-sm text-slate-200">読み込み中...</p>;

  return (
    <div className="mx-auto max-w-md space-y-4">
      <h1 className="text-2xl font-bold">パーソナル指導の申し込み</h1>
      {contracts.length === 0 ? (
        <p className="text-sm text-slate-200">
          まだ申し込みはありません。
          <Link href="/coaches" className="text-blue-300 underline">
            コーチ一覧
          </Link>
          からプランを選べます。
        </p>
      ) : null}
      <ul className="space-y-2">
        {contracts.map((c) => (
          <li key={c.id}>
            <Link
              href={`/coaching/personal/${c.id}`}
              className="flex items-center justify-between gap-3 rounded-xl border border-white/20 glass bg-white/10 p-3"
            >
              <span className="min-w-0 space-y-0.5">
                <span className="block text-sm font-semibold text-white">{c.plan_title}</span>
                <span className="block text-xs text-slate-300">
                  {c.coach?.nickname ?? "名前なし"} · {formatDateTime(c.created_at)}
                </span>
              </span>
              <span className="shrink-0 text-xs text-blue-300">{contractStatusLabel(c)}</span>
            </Link>
          </li>
        ))}
      </ul>
    </div>
  );
}
