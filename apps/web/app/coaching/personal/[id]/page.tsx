"use client";

import { useAuth } from "@/components/auth-provider";
import { CoachAvatar } from "@/components/coach-photo";
import { contractPeriod, ContractDetails } from "@/components/coaching-contracts";
import { CoachingMessages } from "@/components/coaching-messages";
import { apiFetch, ApiRequestError } from "@/lib/api";
import { contractStatusLabels } from "@/lib/coaching-plan";
import { formatDateTime } from "@/lib/format";
import type { CoachingContract } from "@/lib/types";
import { useActionLock } from "@/lib/use-action-lock";
import Link from "next/link";
import { useParams, useRouter } from "next/navigation";
import { useEffect, useState } from "react";

// パーソナル指導の1件。申し込んだ人とコーチが見る。期間中と終わったあとは、やりとり欄を出す。
export default function CoachingContractPage() {
  const { id } = useParams<{ id: string }>();
  const { session, token, loading } = useAuth();
  const router = useRouter();
  const [contract, setContract] = useState<CoachingContract | null>(null);
  const [error, setError] = useState("");
  const { pending, acquire, release } = useActionLock();

  useEffect(() => {
    if (!loading && !session) {
      router.replace(`/login?next=${encodeURIComponent(window.location.pathname)}`);
    }
  }, [loading, session, router]);

  useEffect(() => {
    if (!token) return;
    let cancelled = false;
    apiFetch<{ contract: CoachingContract }>(`/v1/coaching/contracts/${id}`, { token })
      .then((d) => {
        if (!cancelled) setContract(d.contract);
      })
      .catch((err) => {
        if (!cancelled) setError(err instanceof ApiRequestError ? err.message : "読み込みに失敗しました");
      });
    return () => {
      cancelled = true;
    };
  }, [id, token]);

  async function cancel() {
    if (!window.confirm("申し込みを取り下げますか？")) return;
    if (!acquire()) return;
    try {
      const d = await apiFetch<{ contract: CoachingContract }>(`/v1/coaching/contracts/${id}/cancel`, { method: "POST", token });
      setContract((c) => (c ? { ...c, status: d.contract.status } : c));
    } catch (err) {
      setError(err instanceof ApiRequestError ? err.message : "送信に失敗しました");
    } finally {
      release();
    }
  }

  if (error && !contract) return <p className="text-sm text-red-400">{error}</p>;
  if (loading || !session || !contract) return <p className="text-sm text-slate-200">読み込み中...</p>;

  const isCoach = contract.role === "coach";

  return (
    <div className="mx-auto max-w-md space-y-6">
      <div className="space-y-1">
        <h1 className="text-2xl font-bold">パーソナル指導</h1>
        <p className="text-sm font-semibold text-blue-300">{contractStatusLabels[contract.status]}</p>
      </div>

      <section className="space-y-3 rounded-2xl border border-white/20 glass bg-white/10 p-4">
        {isCoach ? (
          <p className="font-bold text-white">{contract.requester?.nickname ?? "名前なし"}さん</p>
        ) : contract.coach ? (
          <div className="flex items-center gap-3">
            <CoachAvatar url={contract.coach.photo_url} name={contract.coach.nickname} />
            <p className="font-bold text-white">{contract.coach.nickname ?? "名前なし"}</p>
          </div>
        ) : null}
        <ContractDetails contract={contract} />
        {contract.status === "applied" ? (
          <p className="text-sm text-slate-200">{formatDateTime(contract.respond_by)}までにコーチから返事が届きます。</p>
        ) : null}
        {contract.status === "accepted" && contract.start_by ? (
          <p className="text-sm text-slate-200">
            {isCoach
              ? `保護者の同意を待っています。${formatDateTime(contract.start_by)}までに同意があると始まります。`
              : `コーチが引き受けました。保護者が${formatDateTime(contract.start_by)}までに、同意のページからこの申し込みに同意すると始まります。保護者に知らせてください。`}
          </p>
        ) : null}
        {contract.starts_at ? <p className="text-sm text-slate-200">期間: {contractPeriod(contract)}</p> : null}
      </section>

      {contract.status === "active" || contract.status === "suspended" || contract.status === "ended" ? (
        <CoachingMessages contract={contract} token={token} onContractChange={setContract} />
      ) : null}

      {!isCoach && contract.status === "applied" ? (
        <button
          type="button"
          disabled={pending}
          onClick={() => void cancel()}
          className="min-h-11 w-full rounded-md border border-white/25 bg-white/10 px-4 text-sm font-semibold text-slate-100 transition active:scale-95 disabled:opacity-50"
        >
          申し込みを取り下げる
        </button>
      ) : null}
      {error ? <p className="text-sm text-red-400">{error}</p> : null}

      <Link href={isCoach ? "/coach" : "/me/coaching/personal"} className="block text-center text-sm text-blue-300 underline">
        {isCoach ? "コーチ画面へ" : "申し込みの一覧へ"}
      </Link>
    </div>
  );
}
