"use client";

import { CoachingSupportPanel } from "@/components/coaching-support";

import { CoachingConsultations } from "@/components/coaching-consultations";

import { useAuth } from "@/components/auth-provider";
import { CoachAvatar } from "@/components/coach-photo";
import { Button } from "@/components/ui/button";
import { contractPeriod, ContractDetails } from "@/components/coaching-contracts";
import { CoachingMessages } from "@/components/coaching-messages";
import { GuardianLinkForm } from "@/components/guardian-link-form";
import { apiFetch, ApiRequestError } from "@/lib/api";
import { contractStatusLabel } from "@/lib/coaching-plan";
import { formatDateTime, formatYen } from "@/lib/format";
import type { CoachingContract } from "@/lib/types";
import { useActionLock } from "@/lib/use-action-lock";
import Link from "next/link";
import { useParams, useRouter, useSearchParams } from "next/navigation";
import { Suspense, useEffect, useState } from "react";

// 支払いのあと戻ってきたとき、Stripe からの知らせが届いて期間が始まるまで、何回か読み直す。
const paidPolls = 10;

// 始まる前の案内。支払いがあるか、18歳未満か、見ているのがコーチかで変わる。
function waitingText(contract: CoachingContract, isCoach: boolean): string {
  const by = formatDateTime(contract.start_by ?? "");
  if (isCoach) {
    if (contract.minor && contract.payment_required) return `保護者の同意とお支払いを待っています。${by}までに済むと始まります。`;
    if (contract.payment_required) return `お支払いを待っています。${by}までに支払いがあると始まります。`;
    return `保護者の同意を待っています。${by}までに同意があると始まります。`;
  }
  if (contract.minor && contract.payment_required) {
    return `コーチが引き受けました。保護者が${by}までに、保護者のページからこの申し込みに同意してお支払いをすると始まります。保護者に知らせてください。`;
  }
  if (contract.payment_required) return `コーチが引き受けました。${by}までにお支払いください。支払った時から期間が始まります。`;
  return `コーチが引き受けました。保護者が${by}までに、同意のページからこの申し込みに同意すると始まります。保護者に知らせてください。`;
}

// パーソナル指導の1件。申し込んだ人とコーチが見る。期間中と終わったあとは、やりとり欄を出す。
function CoachingContractView() {
  const { id } = useParams<{ id: string }>();
  const paid = useSearchParams().get("paid") === "1";
  const [polls, setPolls] = useState(0);
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
  }, [id, token, polls]);

  // 支払いから戻ってきたのにまだ始まっていなければ、少し待って読み直す。
  const waitingForPayment = paid && contract?.status === "accepted" && polls < paidPolls;
  useEffect(() => {
    if (!waitingForPayment) return;
    const t = window.setTimeout(() => setPolls((n) => n + 1), 2000);
    return () => window.clearTimeout(t);
  }, [waitingForPayment, polls]);

  async function pay() {
    if (!acquire()) return;
    setError("");
    try {
      const d = await apiFetch<{ checkout_url: string }>(`/v1/coaching/contracts/${id}/checkout`, { method: "POST", token });
      window.location.href = d.checkout_url;
    } catch (err) {
      setError(err instanceof ApiRequestError ? err.message : "お支払いの画面を開けませんでした");
      release();
    }
  }

  async function cancel() {
    if (!confirm("申し込みを取り下げますか？")) return;
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
        <p className="text-sm font-semibold text-blue-300">{contractStatusLabel(contract)}</p>
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
          <p className="text-sm text-slate-200">{waitingText(contract, isCoach)}</p>
        ) : null}
        {contract.starts_at ? <p className="text-sm text-slate-200">期間: {contractPeriod(contract)}</p> : null}
        {!isCoach && contract.payment ? (
          <p className="text-xs text-slate-300">
            お支払い {formatYen(contract.payment.amount_yen)}（{formatDateTime(contract.payment.paid_at)}）
            {contract.payment.refunded_yen > 0 && contract.payment.refunded_at
              ? ` · 返金 ${formatYen(contract.payment.refunded_yen)}（${formatDateTime(contract.payment.refunded_at)}）`
              : ""}
          </p>
        ) : null}
      </section>

      {!isCoach && !contract.minor && contract.payment_required && contract.status === "accepted" ? (
        waitingForPayment ? (
          <p className="rounded-xl border border-blue-400/40 bg-blue-500/15 p-3 text-sm text-slate-100">
            お支払いを確かめています。このままお待ちください。
          </p>
        ) : (
          <div className="space-y-2">
            <Button type="button" className="w-full" disabled={pending} onClick={() => void pay()}>
              お支払いへ進む（{formatYen(contract.price_yen)}）
            </Button>
            <p className="text-xs text-slate-300">
              カード・Apple Pay・Google Pay・PayPay で払えます。始まってから72時間コーチの書き込みがなければ、全額を返金します。
            </p>
          </div>
        )
      ) : null}

      {!isCoach && contract.minor && contract.status === "accepted" ? <GuardianLinkForm token={token} /> : null}

      {contract.status === "active" || contract.status === "suspended" || contract.status === "ended" || contract.status === "refunded" ? (
        <CoachingMessages contract={contract} token={token} onContractChange={setContract} />
      ) : null}

      {contract.fulfillment_version === 1 ? <CoachingSupportPanel contract={contract} token={token} /> : null}

      <CoachingConsultations rootID={contract.id} kind="contract" token={token} />

      {!isCoach && contract.status === "applied" ? (
        <button
          type="button"
          disabled={pending}
          onClick={() => void cancel()}
          className="min-h-11 w-full rounded-md border border-white/25 bg-white/10 px-4 text-sm font-semibold text-slate-100 transition active:scale-[0.98] disabled:opacity-50"
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

export default function CoachingContractPage() {
  return (
    <Suspense fallback={<p className="text-sm text-slate-200">読み込み中...</p>}>
      <CoachingContractView />
    </Suspense>
  );
}
