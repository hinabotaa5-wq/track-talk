"use client";

import { useAuth } from "@/components/auth-provider";
import { CoachingGate } from "@/components/coaching-gate";
import { Button } from "@/components/ui/button";
import { apiFetch, ApiRequestError } from "@/lib/api";
import Link from "next/link";
import { useCallback, useEffect, useState } from "react";

type Reconciliation = {
  ticket_lot_ledger_mismatches: number;
  paid_purchases_without_lot: number;
  consumed_ticket_earnings_mismatches: number;
  pending_support_refunds: number;
  pending_ticket_refunds: number;
  pending_consultation_refunds: number;
  unpaid_coach_earnings: number;
};

const checks: { key: keyof Reconciliation; label: string }[] = [
  { key: "ticket_lot_ledger_mismatches", label: "チケット残高と台帳の不一致" },
  { key: "paid_purchases_without_lot", label: "支払い済みでチケットの付与記録がない購入" },
  { key: "consumed_ticket_earnings_mismatches", label: "チケット利用確定とコーチ報酬の不一致" },
];
const pendingItems: { key: keyof Reconciliation; label: string }[] = [
  { key: "pending_ticket_refunds", label: "チケットの返金待ち" },
  { key: "pending_consultation_refunds", label: "追加相談の返金待ち" },
  { key: "pending_support_refunds", label: "大会サポート・対面の返金待ち／失敗" },
  { key: "unpaid_coach_earnings", label: "コーチへの送金待ち" },
];

function ReconciliationAdmin() {
  const { token } = useAuth();
  const [data, setData] = useState<Reconciliation | null>(null);
  const [checkedAt, setCheckedAt] = useState("");
  const [error, setError] = useState("");
  const [loading, setLoading] = useState(true);
  const load = useCallback(async (signal: AbortSignal) => {
    if (!token) return;
    try {
      const result = await apiFetch<Reconciliation>("/v1/admin/coaching/reconciliation", { token, signal });
      if (!signal.aborted) {
        setData(result);
        setCheckedAt(new Date().toLocaleString("ja-JP", { timeZone: "Asia/Tokyo" }));
      }
    } catch (err) {
      if (!signal.aborted) setError(err instanceof ApiRequestError ? err.message : "照合できませんでした");
    } finally {
      if (!signal.aborted) setLoading(false);
    }
  }, [token]);
  useEffect(() => {
    const controller = new AbortController();
    if (!token) return () => controller.abort();
    apiFetch<Reconciliation>("/v1/admin/coaching/reconciliation", { token, signal: controller.signal })
      .then((result) => {
        if (!controller.signal.aborted) {
          setData(result);
          setCheckedAt(new Date().toLocaleString("ja-JP", { timeZone: "Asia/Tokyo" }));
        }
      })
      .catch((err) => {
        if (!controller.signal.aborted) setError(err instanceof ApiRequestError ? err.message : "照合できませんでした");
      })
      .finally(() => { if (!controller.signal.aborted) setLoading(false); });
    return () => controller.abort();
  }, [token]);

  return <div className="mx-auto max-w-2xl space-y-5">
    <h1 className="text-xl font-bold">コーチングの会計照合</h1>
    <Link href="/admin" className="text-sm text-blue-300 underline">運営画面に戻る</Link>
    <p className="text-sm text-slate-300">日次で確認する、アプリ内の会計記録の照合です。Stripeでの支払い・返金・送金の結果とも照らし合わせてください。</p>
    <Button type="button" disabled={loading || !token} onClick={() => { setLoading(true); setError(""); setData(null); void load(new AbortController().signal); }}>{loading ? "照合中..." : "再照合する"}</Button>
    {error ? <p role="alert" className="text-sm text-red-400">{error}</p> : null}
    {data ? <>
      <p className="text-xs text-slate-300">確認日時（日本時間）：{checkedAt}</p>
      <section className="space-y-3 rounded-xl border border-white/20 p-4">
        <h2 className="font-semibold">不一致の確認</h2>
        <dl className="space-y-3">{checks.map(({ key, label }) => <div key={key} className={data[key] ? "text-amber-300" : "text-slate-200"}>
          <dt className="text-sm">{label}</dt><dd className="font-semibold">{data[key]}件</dd>
        </div>)}</dl>
        {checks.some(({ key }) => data[key] > 0) ? <p role="alert" className="text-sm text-amber-300">不一致があります。記録を確認してから返金や送金を進めてください。</p> : <p className="text-sm">上記の照合では不一致は見つかりませんでした。</p>}
      </section>
      <section className="space-y-3 rounded-xl border border-white/20 p-4">
        <h2 className="font-semibold">処理待ちの確認</h2>
        <p className="text-sm text-slate-300">処理待ちの件数は不一致ではありません。前日の記録と比べ、同じ返金が残り続けていないか確認してください。</p>
        <dl className="space-y-3">{pendingItems.map(({ key, label }) => <div key={key}><dt className="text-sm">{label}</dt><dd>{data[key]}件</dd></div>)}</dl>
      </section>
    </> : loading ? <p role="status">照合しています...</p> : null}
    <nav className="flex flex-wrap gap-4 text-sm text-blue-300 underline">
      <Link href="/admin/coaching/tickets">チケット購入・返金管理</Link>
      <Link href="/admin/coaching/consultations">追加相談・返金管理</Link>
    </nav>
  </div>;
}

export default function Page() { return <CoachingGate><ReconciliationAdmin /></CoachingGate>; }
