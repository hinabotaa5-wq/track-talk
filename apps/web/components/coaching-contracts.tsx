"use client";

import { Button } from "@/components/ui/button";
import { apiFetch, ApiRequestError } from "@/lib/api";
import { coachEventLabels } from "@/lib/coach-recruit";
import {
  contractConsentItems,
  contractStatusLabel,
  planDurationLabel,
  planKindLabels,
  type ContractConsentKey,
} from "@/lib/coaching-plan";
import { formatDateTime, formatYen } from "@/lib/format";
import type { CoachingContract } from "@/lib/types";
import { useActionLock } from "@/lib/use-action-lock";
import Link from "next/link";
import { useCallback, useEffect, useState } from "react";

// パーソナル指導の申し込み。同意のチェック、申し込みの中身、コーチ画面の「申し込み・期間中」。

export type ContractConsents = Record<ContractConsentKey, boolean>;
export const noConsents: ContractConsents = { agree_report: false, agree_detection: false };

// ContractConsentChecks は、契約を始めるときの2つの同意。最初はどちらもチェックしない。
export function ContractConsentChecks({
  value,
  onChange,
  errors,
}: {
  value: ContractConsents;
  onChange: (v: ContractConsents) => void;
  errors?: Record<string, string>;
}) {
  return (
    <fieldset className="space-y-2">
      <legend className="text-sm font-semibold">やりとりについての同意</legend>
      {(Object.keys(contractConsentItems) as ContractConsentKey[]).map((key) => (
        <div key={key}>
          <label className="flex items-start gap-2 text-sm text-slate-100">
            <input
              type="checkbox"
              className="mt-1 size-4 shrink-0"
              checked={value[key]}
              onChange={(e) => onChange({ ...value, [key]: e.target.checked })}
            />
            <span>{contractConsentItems[key]}</span>
          </label>
          {errors?.[key] ? <p className="text-sm text-red-400">{errors[key]}</p> : null}
        </div>
      ))}
    </fieldset>
  );
}

// ContractDetails は、申し込みの中身（プランと、利用者が書いたこと）。
export function ContractDetails({ contract }: { contract: CoachingContract }) {
  return (
    <dl className="space-y-1.5 text-sm">
      <div>
        <dt className="text-xs text-slate-300">プラン</dt>
        <dd className="text-slate-100">
          {contract.plan_title}（{planKindLabels[contract.plan_kind]} · {planDurationLabel(contract.plan_kind, contract.duration_days)} ·{" "}
          {formatYen(contract.price_yen)}）
        </dd>
      </div>
      <div>
        <dt className="text-xs text-slate-300">種目</dt>
        <dd className="text-slate-100">{coachEventLabels[contract.event]}</dd>
      </div>
      <div>
        <dt className="text-xs text-slate-300">目標</dt>
        <dd className="whitespace-pre-wrap text-slate-100">{contract.goal}</dd>
      </div>
      <div>
        <dt className="text-xs text-slate-300">いまの記録</dt>
        <dd className="whitespace-pre-wrap text-slate-100">{contract.current_record}</dd>
      </div>
      <div>
        <dt className="text-xs text-slate-300">相談したいこと</dt>
        <dd className="whitespace-pre-wrap text-slate-100">{contract.message}</dd>
      </div>
    </dl>
  );
}

// contractPeriod は、期間中・終わったものの期間の表示。
export function contractPeriod(c: CoachingContract): string {
  if (!c.starts_at || !c.ends_at) return "";
  return `${formatDateTime(c.starts_at)} 〜 ${formatDateTime(c.ends_at)}`;
}

function ApplicationCard({ contract, token, onDone }: { contract: CoachingContract; token: string | null; onDone: () => void }) {
  const [consents, setConsents] = useState<ContractConsents>(noConsents);
  const [fields, setFields] = useState<Record<string, string>>({});
  const [error, setError] = useState("");
  const { pending, acquire, release } = useActionLock();

  async function answer(action: "accept" | "decline") {
    if (action === "decline" && !window.confirm("この申し込みを断りますか？")) return;
    if (!acquire()) return;
    setError("");
    setFields({});
    try {
      await apiFetch(`/v1/coach/contracts/${contract.id}/${action}`, {
        method: "POST",
        token,
        body: action === "accept" ? JSON.stringify(consents) : undefined,
      });
      onDone();
    } catch (err) {
      if (err instanceof ApiRequestError && err.fields) setFields(err.fields);
      else setError(err instanceof ApiRequestError ? err.message : "送信に失敗しました");
    } finally {
      release();
    }
  }

  return (
    <li className="space-y-3 rounded-xl border border-blue-400/40 bg-blue-500/15 p-3">
      <div className="flex items-center justify-between gap-2">
        <p className="font-semibold text-white">{contract.requester?.nickname ?? "名前なし"}さん</p>
        {contract.minor ? <span className="rounded-full bg-amber-500/30 px-2 py-0.5 text-xs text-white">18歳未満</span> : null}
      </div>
      <ContractDetails contract={contract} />
      <p className="text-sm font-semibold text-white">{formatDateTime(contract.respond_by)}までに返事をしてください</p>
      {contract.minor ? (
        <p className="text-xs text-slate-200">
          18歳未満の人です。引き受けたあと、保護者が72時間以内にこの申し込みに同意すると始まります。保護者もやりとりに参加します。
        </p>
      ) : null}
      <ContractConsentChecks value={consents} onChange={setConsents} errors={fields} />
      <div className="flex gap-2">
        <Button
          type="button"
          className="flex-1"
          disabled={pending || !consents.agree_report || !consents.agree_detection}
          onClick={() => void answer("accept")}
        >
          引き受ける
        </Button>
        <button
          type="button"
          disabled={pending}
          onClick={() => void answer("decline")}
          className="min-h-11 flex-1 rounded-md border border-white/25 bg-white/10 px-4 text-sm font-semibold text-slate-100 transition active:scale-95 disabled:opacity-50"
        >
          断る
        </button>
      </div>
      {error ? <p className="text-sm text-red-400">{error}</p> : null}
    </li>
  );
}

// CoachContractsSection は、コーチ画面の「パーソナル指導の申し込み」と「期間中」。
export function CoachContractsSection({ token }: { token: string | null }) {
  const [contracts, setContracts] = useState<CoachingContract[] | null>(null);
  const [error, setError] = useState("");
  const [reload, setReload] = useState(0);

  const fetchContracts = useCallback(
    () => apiFetch<{ contracts: CoachingContract[] }>("/v1/coach/contracts", { token }),
    [token],
  );

  useEffect(() => {
    let cancelled = false;
    fetchContracts()
      .then((d) => {
        if (!cancelled) setContracts(d.contracts);
      })
      .catch(() => {
        if (!cancelled) setError("申し込みを読み込めませんでした");
      });
    return () => {
      cancelled = true;
    };
  }, [fetchContracts, reload]);

  if (!contracts) return error ? <p className="text-sm text-red-400">{error}</p> : null;

  const applied = contracts.filter((c) => c.status === "applied");
  const active = contracts.filter((c) => c.status === "accepted" || c.status === "active" || c.status === "suspended");

  return (
    <>
      <section className="space-y-2">
        <h2 className="font-bold">パーソナル指導の申し込み</h2>
        {applied.length === 0 ? <p className="text-sm text-slate-300">いま届いている申し込みはありません</p> : null}
        <ul className="space-y-2">
          {applied.map((c) => (
            <ApplicationCard key={c.id} contract={c} token={token} onDone={() => setReload((n) => n + 1)} />
          ))}
        </ul>
      </section>
      <section className="space-y-2">
        <h2 className="font-bold">パーソナル指導の期間中</h2>
        {active.length === 0 ? <p className="text-sm text-slate-300">期間中の人はいません</p> : null}
        <ul className="space-y-2">
          {active.map((c) => (
            <li key={c.id}>
              <Link
                href={`/coaching/personal/${c.id}`}
                className="block space-y-0.5 rounded-xl border border-white/20 glass bg-white/10 p-3"
              >
                <span className="block text-sm font-semibold text-white">
                  {c.requester?.nickname ?? "名前なし"}さん · {c.plan_title}
                </span>
                <span className="block text-xs text-slate-300">
                  {c.status === "active" ? contractPeriod(c) : contractStatusLabel(c)}
                </span>
              </Link>
            </li>
          ))}
        </ul>
      </section>
    </>
  );
}

