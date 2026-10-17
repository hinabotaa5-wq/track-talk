"use client";

import { apiFetch, ApiRequestError } from "@/lib/api";
import { contractStatusLabels, formatPlanPrice } from "@/lib/coaching-plan";
import { formatDateTime } from "@/lib/format";
import type { AdminCoachingContract, CoachingBlockedAccount, CoachingContractStatus } from "@/lib/types";
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
    if (!window.confirm(ask)) return;
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
  if (!data) return <p className="text-sm text-slate-200">読み込み中...</p>;

  function setStatus(id: string, status: CoachingContractStatus) {
    setData((d) => (d ? { ...d, contracts: d.contracts.map((c) => (c.id === id ? { ...c, status } : c)) } : d));
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
                {contractStatusLabels[c.status]} · {formatPlanPrice(c.price_yen)} · 申し込み {formatDateTime(c.created_at)}
                {c.minor ? " · 18歳未満" : ""}
              </p>
              <p className="font-semibold text-white">{c.plan_title}</p>
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
              <SuspendToggle id={c.id} status={c.status} token={token} onChange={(s) => setStatus(c.id, s)} />
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
