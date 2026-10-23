"use client";

import { CoachAvatar } from "@/components/coach-photo";
import { ContractConsentChecks, ContractDetails, contractPeriod, noConsents, type ContractConsents } from "@/components/coaching-contracts";
import { Button } from "@/components/ui/button";
import { apiFetch, ApiRequestError } from "@/lib/api";
import { contractStatusLabel } from "@/lib/coaching-plan";
import { formatDateTime, formatYen } from "@/lib/format";
import type { CoachingContract } from "@/lib/types";
import { useActionLock } from "@/lib/use-action-lock";
import Link from "next/link";
import { useCallback, useEffect, useState } from "react";

// 保護者のページの「パーソナル指導」。コーチが引き受けた申し込みに契約ごとに同意し、期間中はやりとりに参加する。
// 支払いがある申し込みは、同意したら Stripe の支払い画面へ進み、支払いが済んだ時から期間が始まる。

function ConsentCard({ contract, token, onDone }: { contract: CoachingContract; token: string; onDone: () => void }) {
  const [consents, setConsents] = useState<ContractConsents>(noConsents);
  const [fields, setFields] = useState<Record<string, string>>({});
  const [error, setError] = useState("");
  const { pending, acquire, release } = useActionLock();

  async function answer(action: "consent" | "decline") {
    if (action === "decline" && !confirm("同意しない場合、このパーソナル指導は始まりません。よろしいですか？")) return;
    if (!acquire()) return;
    setError("");
    setFields({});
    try {
      const d = await apiFetch<{ checkout_url?: string }>(`/v1/guardian/${encodeURIComponent(token)}/contracts/${contract.id}/${action}`, {
        method: "POST",
        body: action === "consent" ? JSON.stringify(consents) : undefined,
      });
      if (d.checkout_url) {
        window.location.href = d.checkout_url;
        return;
      }
      onDone();
    } catch (err) {
      if (err instanceof ApiRequestError && err.fields) setFields(err.fields);
      else setError(err instanceof ApiRequestError ? err.message : "送信に失敗しました");
    } finally {
      release();
    }
  }

  async function pay() {
    if (!acquire()) return;
    setError("");
    try {
      const d = await apiFetch<{ checkout_url: string }>(`/v1/guardian/${encodeURIComponent(token)}/contracts/${contract.id}/checkout`, {
        method: "POST",
      });
      window.location.href = d.checkout_url;
    } catch (err) {
      setError(err instanceof ApiRequestError ? err.message : "お支払いの画面を開けませんでした");
      release();
    }
  }

  const by = contract.start_by ? formatDateTime(contract.start_by) : "";
  if (contract.payment_required && contract.guardian_consented) {
    return (
      <div className="space-y-3">
        <ContractDetails contract={contract} />
        <p className="text-sm font-semibold text-white">同意は済んでいます。{by}までにお支払いを済ませると、その時から期間が始まります。</p>
        <Button type="button" className="w-full" disabled={pending} onClick={() => void pay()}>
          お支払いへ進む（{formatYen(contract.price_yen)}）
        </Button>
        {error ? <p className="text-sm text-red-400">{error}</p> : null}
      </div>
    );
  }

  return (
    <div className="space-y-3">
      <ContractDetails contract={contract} />
      {by ? (
        <p className="text-sm font-semibold text-white">
          {contract.payment_required
            ? `${by}までに同意してお支払いを済ませると、その時から期間が始まります。`
            : `${by}までに同意すると、その時から期間が始まります。`}
        </p>
      ) : null}
      <ul className="list-disc space-y-1 pl-5 text-xs text-slate-200">
        <li>期間中は、お子さんとコーチのやりとりをこのページから読めて、書き込めます。新しいメッセージの通知は届きません。</li>
        <li>連絡先の交換と、直接会う約束は禁止です。気になるメッセージは「通報する」から運営に知らせてください。</li>
        {contract.payment_required ? (
          <li>
            料金は{formatYen(contract.price_yen)}で、同意のあとお支払いの画面（カード・Apple Pay・Google Pay・PayPay）に進みます。自動更新はありません。始まってから72時間コーチの書き込みがなければ、全額を返金します。
          </li>
        ) : (
          <li>いまはお試し期間のため、お支払いはありません。</li>
        )}
      </ul>
      <ContractConsentChecks value={consents} onChange={setConsents} errors={fields} />
      <div className="flex gap-2">
        <Button
          type="button"
          className="flex-1"
          disabled={pending || !consents.agree_report || !consents.agree_detection}
          onClick={() => void answer("consent")}
        >
          {contract.payment_required ? "同意してお支払いへ進む" : "同意して始める"}
        </Button>
        <button
          type="button"
          disabled={pending}
          onClick={() => void answer("decline")}
          className="min-h-11 flex-1 rounded-md border border-white/25 bg-white/10 px-4 text-sm font-semibold text-slate-100 transition active:scale-[0.98] disabled:opacity-50"
        >
          同意しない
        </button>
      </div>
      {error ? <p className="text-sm text-red-400">{error}</p> : null}
    </div>
  );
}

// GuardianPersonalSection は、保護者のページに出すお子さんのパーソナル指導の一覧。
export function GuardianPersonalSection({ token }: { token: string }) {
  const [contracts, setContracts] = useState<CoachingContract[] | null>(null);
  const [error, setError] = useState("");
  const [reload, setReload] = useState(0);

  const fetchContracts = useCallback(
    () => apiFetch<{ contracts: CoachingContract[] }>(`/v1/guardian/${encodeURIComponent(token)}/contracts`),
    [token],
  );

  useEffect(() => {
    let cancelled = false;
    fetchContracts()
      .then((d) => {
        if (!cancelled) setContracts(d.contracts);
      })
      .catch(() => {
        if (!cancelled) setError("パーソナル指導を読み込めませんでした");
      });
    return () => {
      cancelled = true;
    };
  }, [fetchContracts, reload]);

  if (!contracts) return error ? <p className="text-sm text-red-400">{error}</p> : null;
  if (contracts.length === 0) return null;

  return (
    <section className="space-y-3">
      <div className="space-y-1">
        <h2 className="text-lg font-bold">パーソナル指導</h2>
        <p className="text-sm text-slate-300">
          コーチが期間中にお子さんの練習を見る指導です。コーチが引き受けたあと、申し込みごとに保護者の同意が必要です。
        </p>
      </div>
      <ul className="space-y-3">
        {contracts.map((c) => (
          <li key={c.id} className="space-y-3 rounded-2xl border border-white/20 glass bg-white/10 p-4">
            <div className="flex items-center gap-3">
              {c.coach ? <CoachAvatar url={c.coach.photo_url} name={c.coach.nickname} size={44} /> : null}
              <div className="min-w-0">
                <p className="font-semibold text-white">{c.coach?.nickname ?? "コーチ"}</p>
                <p className="text-xs text-blue-300">{contractStatusLabel(c)}</p>
              </div>
            </div>
            {c.status === "accepted" ? (
              <ConsentCard contract={c} token={token} onDone={() => setReload((n) => n + 1)} />
            ) : (
              <>
                <p className="text-sm text-slate-100">{c.plan_title}</p>
                {c.starts_at ? <p className="text-xs text-slate-300">期間: {contractPeriod(c)}</p> : null}
                {c.status === "active" || c.status === "suspended" || c.status === "ended" || c.status === "refunded" ? (
                  <Link
                    href={`/guardian/${encodeURIComponent(token)}/personal/${c.id}`}
                    className="block text-sm text-blue-300 underline"
                  >
                    やりとりを見る
                  </Link>
                ) : null}
              </>
            )}
          </li>
        ))}
      </ul>
    </section>
  );
}
