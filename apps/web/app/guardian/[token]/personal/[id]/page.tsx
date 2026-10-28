"use client";

import { PageLoading } from "@/components/page-loading";

import { CoachingSupportPanel } from "@/components/coaching-support";
import { CoachingInPersonPanel } from "@/components/coaching-in-person";

import { CoachingConsultations } from "@/components/coaching-consultations";

import { CoachAvatar } from "@/components/coach-photo";
import { ContractDetails, contractPeriod } from "@/components/coaching-contracts";
import { CoachingMessages } from "@/components/coaching-messages";
import { apiFetch, ApiRequestError } from "@/lib/api";
import { contractStatusLabel } from "@/lib/coaching-plan";
import type { CoachingContract } from "@/lib/types";
import Link from "next/link";
import { useParams } from "next/navigation";
import { useEffect, useState } from "react";

// 保護者がリンクから開く、お子さんのパーソナル指導のやりとり。ログインはいらない。
export default function GuardianContractPage() {
  const { token, id } = useParams<{ token: string; id: string }>();
  const [contract, setContract] = useState<CoachingContract | null>(null);
  const [error, setError] = useState("");

  useEffect(() => {
    let cancelled = false;
    apiFetch<{ contract: CoachingContract }>(`/v1/guardian/${encodeURIComponent(token)}/contracts/${id}`)
      .then((d) => {
        if (!cancelled) setContract(d.contract);
      })
      .catch((err) => {
        if (!cancelled) setError(err instanceof ApiRequestError ? err.message : "読み込みに失敗しました");
      });
    return () => {
      cancelled = true;
    };
  }, [token, id]);

  if (error) return <p className="text-sm text-red-400">{error}</p>;
  if (!contract) return <PageLoading label="読み込み中..." />;

  return (
    <div className="mx-auto max-w-md space-y-6">
      <div className="space-y-1">
        <h1 className="text-2xl font-bold">パーソナル指導（保護者）</h1>
        <p className="text-sm font-semibold text-blue-300">{contractStatusLabel(contract)}</p>
      </div>
      <section className="space-y-3 rounded-2xl border border-white/20 glass bg-white/10 p-4">
        {contract.coach ? (
          <div className="flex items-center gap-3">
            <CoachAvatar url={contract.coach.photo_url} name={contract.coach.nickname} />
            <p className="font-bold text-white">{contract.coach.nickname ?? "名前なし"}</p>
          </div>
        ) : null}
        <ContractDetails contract={contract} />
        {contract.starts_at ? <p className="text-sm text-slate-200">期間: {contractPeriod(contract)}</p> : null}
      </section>
      {contract.status === "active" || contract.status === "suspended" || contract.status === "ended" || contract.status === "refunded" ? (
        <CoachingMessages contract={contract} token={null} guardianToken={token} onContractChange={setContract} />
      ) : null}
      {contract.fulfillment_version === 1 ? <CoachingSupportPanel contract={contract} token={null} guardianToken={token} /> : null}
      {contract.fulfillment_version === 2 ? <CoachingInPersonPanel contract={contract} token={null} guardianToken={token} /> : null}

      <CoachingConsultations rootID={contract.id} kind="contract" token={null} guardianToken={token} />

      <Link href={`/guardian/${encodeURIComponent(token)}`} className="block text-center text-sm text-blue-300 underline">
        保護者のページへ
      </Link>
    </div>
  );
}
