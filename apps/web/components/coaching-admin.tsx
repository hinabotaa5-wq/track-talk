"use client";

import { PageLoading } from "@/components/page-loading";

import { apiFetch, ApiRequestError } from "@/lib/api";
import { contractStatusLabel } from "@/lib/coaching-plan";
import { formatDateTime, formatYen } from "@/lib/format";
import type { AdminCoachingContract, CoachingBlockedAccount, CoachingContractStatus, CoachingPayment } from "@/lib/types";
import { useActionLock } from "@/lib/use-action-lock";
import { useCallback, useEffect, useState } from "react";

// 運営のパーソナル指導。一覧には会話の中身を出さない。会話は通報の一覧から、通報を指定して開く。

// SuspendToggle は、期間中のやりとりを止める・止めていたものを再開するボタン。
export function SuspendToggle({
  id,
  status,
  token,
  onChange,
}: {
  id: string;
  status: CoachingContractStatus;
  token: string | null;
  onChange: (status: CoachingContractStatus) => void;
}) {
  const [error, setError] = useState("");
  const { pending, acquire, release } = useActionLock();
  if (status !== "active" && status !== "suspended") return null;
  const suspend = status === "active";

  async function run() {
    const ask = suspend
      ? "このやりとりを止めますか？両方に知らせ、再開するまで送れなくなります。"
      : "やりとりを再開しますか？止めていた日数の分、期間を延ばします。";
    if (!confirm(ask)) return;
    if (!acquire()) return;
    setError("");
    try {
      await apiFetch(`/v1/admin/coaching/contracts/${id}/${suspend ? "suspend" : "resume"}`, { method: "POST", token });
      onChange(suspend ? "suspended" : "active");
    } catch (err) {
      setError(err instanceof ApiRequestError ? err.message : "できませんでした");
    } finally {
      release();
    }
  }

  return (
    <span className="inline-flex items-center gap-2">
      <button type="button" disabled={pending} onClick={() => void run()} className={`text-sm underline ${suspend ? "text-red-300" : "text-blue-300"}`}>
        {suspend ? "やりとりを止める" : "再開する"}
      </button>
      {error ? <span className="text-xs text-red-400">{error}</span> : null}
    </span>
  );
}

const paymentLabels: Record<AdminCoachingContract["payment_status"], string> = {
  "": "まだ払われていません",
  paid: "支払い済み",
  refunded: "全額返金済み",
  partially_refunded: "一部返金済み",
};

// RefundButtons は、運営の返金。全額か、期間中なら残りの日数分。返金すると期間中のものは終わる。
function RefundButtons({
  contract,
  token,
  onChange,
}: {
  contract: AdminCoachingContract;
  token: string | null;
  onChange: (c: Partial<AdminCoachingContract>) => void;
}) {
  const [error, setError] = useState("");
  const { pending, acquire, release } = useActionLock();
  if (contract.payment_status !== "paid") return null;
  const running = contract.status === "active" || contract.status === "suspended";

  async function refund(kind: "full" | "remaining") {
    const ask =
      kind === "full"
        ? `${formatYen(contract.price_yen)}を全額返金しますか？${running ? "期間中のものは、ここで終わります。" : ""}`
        : contract.fulfillment_version === 1 ? "未提供の動画回答・通話の単位数に応じて返金し、ここで終了しますか？" : "残りの日数分を返金して、ここで終わりにしますか？";
    if (!confirm(ask)) return;
    if (!acquire()) return;
    setError("");
    try {
      const d = await apiFetch<{ contract: { status: CoachingContractStatus }; payment: CoachingPayment }>(
        `/v1/admin/coaching/contracts/${contract.id}/refund`,
        { method: "POST", token, body: JSON.stringify({ kind }) },
      );
      onChange({ status: d.contract.status, payment_status: d.payment.status, refunded_yen: d.payment.refunded_yen });
    } catch (err) {
      setError(err instanceof ApiRequestError ? err.message : "返金できませんでした");
    } finally {
      release();
    }
  }

  return (
    <span className="flex flex-wrap items-center gap-3">
      <button type="button" disabled={pending} onClick={() => void refund("full")} className="text-sm text-red-300 underline">
        全額を返金
      </button>
      {running || contract.fulfillment_version === 1 ? (
        <button type="button" disabled={pending} onClick={() => void refund("remaining")} className="text-sm text-red-300 underline">
          {contract.fulfillment_version === 1 ? "未提供分を返金・再試行" : "残りの日数分を返金"}
        </button>
      ) : null}
      {error ? <span className="text-xs text-red-400">{error}</span> : null}
    </span>
  );
}

export function CoachingContractsAdmin({ token }: { token: string | null }) {
  const [data, setData] = useState<{ contracts: AdminCoachingContract[]; blocked_accounts: CoachingBlockedAccount[] } | null>(null);
  const [error, setError] = useState("");

  const fetchData = useCallback(
    () =>
      apiFetch<{ contracts: AdminCoachingContract[]; blocked_accounts: CoachingBlockedAccount[] }>("/v1/admin/coaching/contracts", {
        token,
      }),
    [token],
  );

  useEffect(() => {
    let cancelled = false;
    fetchData()
      .then((d) => {
        if (!cancelled) setData(d);
      })
      .catch(() => {
        if (!cancelled) setError("取得に失敗しました");
      });
    return () => {
      cancelled = true;
    };
  }, [fetchData]);

  if (error) return <p className="text-sm text-red-400">{error}</p>;
  if (!data) return <PageLoading label="読み込み中..." />;

  function patch(id: string, change: Partial<AdminCoachingContract>) {
    setData((d) => (d ? { ...d, contracts: d.contracts.map((c) => (c.id === id ? { ...c, ...change } : c)) } : d));
  }

  return (
    <div className="space-y-6">
      <p className="text-xs text-slate-300">
        会話の中身はここには出しません。通報があった契約だけ、「通報」タブから開けます（開いた人と日時は消せない形で残ります）。
      </p>
      <section className="space-y-2">
        <h2 className="font-bold">契約（新しい順に100件）</h2>
        {data.contracts.length === 0 ? <p className="text-sm text-slate-300">まだありません</p> : null}
        <ul className="space-y-2">
          {data.contracts.map((c) => (
            <li key={c.id} className="space-y-1 rounded-xl border border-white/20 glass bg-white/10 p-3 text-sm">
              <p className="text-xs text-slate-200">
                {contractStatusLabel(c)} · {formatYen(c.price_yen)} · 申し込み {formatDateTime(c.created_at)}
                {c.minor ? " · 18歳未満" : ""}
              </p>
              <p className="font-semibold text-white">{c.plan_title}</p>
              {c.fulfillment_version === 1 ? <p className="text-xs text-slate-300">提供済み {c.delivered_units} / {c.total_units}単位 · 精算 {c.fulfillment_state}</p> : null}
              <p className="text-slate-200">
                {c.requester_nickname ?? "名前なし"}さん → コーチ {c.coach_nickname ?? "名前なし"}
              </p>
              {c.starts_at && c.ends_at ? (
                <p className="text-xs text-slate-300">
                  期間 {formatDateTime(c.starts_at)} 〜 {formatDateTime(c.ends_at)}
                </p>
              ) : null}
              <p className={`text-xs ${c.report_count > 0 ? "text-amber-200" : "text-slate-300"}`}>
                通報 {c.report_count}件 · 連絡先で止めた回数 利用者 {c.requester_blocked} / コーチ {c.coach_blocked}
              </p>
              {c.payment_required ? (
                <p className="text-xs text-slate-300">
                  {paymentLabels[c.payment_status]}
                  {c.refunded_yen > 0 ? `（返金 ${formatYen(c.refunded_yen)}）` : ""}
                </p>
              ) : null}
              <div className="flex flex-wrap items-center gap-3">
                <SuspendToggle id={c.id} status={c.status} token={token} onChange={(status) => patch(c.id, { status })} />
                <RefundButtons contract={c} token={token} onChange={(change) => patch(c.id, change)} />
              </div>
            </li>
          ))}
        </ul>
      </section>
      <section className="space-y-2">
        <h2 className="font-bold">連絡先の検出で止めた回数が多いアカウント</h2>
        <p className="text-xs text-slate-300">止めた文章は保存していません。回数だけです。</p>
        {data.blocked_accounts.length === 0 ? <p className="text-sm text-slate-300">いまはありません</p> : null}
        <ul className="space-y-1">
          {data.blocked_accounts.map((b) => (
            <li key={b.profile_id} className="flex items-center justify-between gap-2 rounded-lg border border-white/15 px-3 py-2 text-sm">
              <span className="text-slate-100">{b.nickname ?? "名前なし"}</span>
              <span className="text-xs text-slate-300">
                {b.blocked}回 · 最後 {formatDateTime(b.last_blocked_at)}
              </span>
            </li>
          ))}
        </ul>
      </section>
    </div>
  );
}
