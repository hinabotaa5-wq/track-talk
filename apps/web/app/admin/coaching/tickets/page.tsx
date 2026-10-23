"use client";

import { useAuth } from "@/components/auth-provider";
import { CoachingGate } from "@/components/coaching-gate";
import { useConfirm } from "@/components/confirm-dialog";
import { Button } from "@/components/ui/button";
import { apiFetch, ApiRequestError } from "@/lib/api";
import { ticketStatusLabels, type TicketPurchase } from "@/lib/coaching-tickets";
import { formatDateTime, formatYen } from "@/lib/format";
import { useActionLock } from "@/lib/use-action-lock";
import Link from "next/link";
import { useCallback, useEffect, useState } from "react";

type Data = {
  purchases: (TicketPurchase & { nickname: string | null; refund_pending: boolean })[] | null;
  balance_issues: string[] | null;
  pending_refunds: { id: string; purchase_id: string; amount_yen: number; quantity: number }[] | null;
};

function TicketAdmin() {
  const { token } = useAuth();
  const [data, setData] = useState<Data | null>(null);
  const [error, setError] = useState("");
  const [message, setMessage] = useState("");
  const confirmDialog = useConfirm();
  const { pending, acquire, release } = useActionLock();
  const load = useCallback(async () => setData(await apiFetch<Data>("/v1/admin/coaching/tickets", { token })), [token]);
  useEffect(() => {
    if (!token) return;
    let cancelled = false;
    apiFetch<Data>("/v1/admin/coaching/tickets", { token }).then((d) => { if (!cancelled) setData(d); })
      .catch((err) => { if (!cancelled) setError(err instanceof ApiRequestError ? err.message : "読み込めませんでした"); });
    return () => { cancelled = true; };
  }, [token]);

  async function refund(p: TicketPurchase) {
    if (!(await confirmDialog({ title: "未使用分を返金しますか？", body: `未使用・未予約の${p.available}枚を返金します。割引購入は購入額に応じて計算します。`, confirmLabel: "返金する" }))) return;
    if (!acquire()) return;
    setError(""); setMessage("");
    try {
      const result = await apiFetch<{ message: string }>(`/v1/admin/coaching/tickets/${p.id}/refund`, { method: "POST", token });
      setMessage(result.message);
      await load();
    } catch (err) { setError(err instanceof ApiRequestError ? err.message : "返金を受け付けられませんでした"); }
    finally { release(); }
  }

  return <div className="mx-auto max-w-2xl space-y-4">
    <h1 className="text-xl font-bold">チケット購入・返金管理</h1>
    <Link href="/admin" className="text-sm text-blue-300 underline">運営画面に戻る</Link>
    {error ? <p role="alert" className="text-sm text-red-400">{error}</p> : null}
    {message ? <p role="status" className="text-sm text-blue-200">{message}</p> : null}
    <Button type="button" disabled={pending} onClick={() => void load().catch(() => setError("読み込めませんでした"))}>再読み込み</Button>
    {data ? <>
      <p className={(data.balance_issues ?? []).length ? "text-amber-300" : "text-slate-200"}>残高と台帳の不一致：{data.balance_issues?.length ?? 0}件</p>
      {(data.balance_issues ?? []).map((id) => <p key={id} className="break-all text-xs text-amber-300">要確認の購入：{id}</p>)}
      <p className="text-sm">処理中の返金：{data.pending_refunds?.length ?? 0}件（定期処理で照合・再実行します）</p>
      <ul className="space-y-3">{(data.purchases ?? []).map((p) => <li key={p.id} className="space-y-2 rounded-xl border border-white/20 p-3 text-sm">
        <p className="font-semibold">{p.nickname ?? "退会した利用者"}：{p.quantity}枚・{formatYen(p.amount_yen)}・{ticketStatusLabels[p.status]}</p>
        <p>未使用 {p.available}枚／確保中 {p.reserved}枚／返金済み {formatYen(p.refunded_yen)}</p>
        <p className="break-all text-xs text-slate-300">{p.id} · {formatDateTime(p.created_at)}</p>
        <Button type="button" disabled={pending || (!p.refund_pending && p.available === 0)} onClick={() => void refund(p)}>
          {p.refund_pending ? "返金状況を照合・再試行" : "未使用分を返金する"}
        </Button>
      </li>)}</ul>
    </> : <p>読み込み中...</p>}
  </div>;
}

export default function Page() { return <CoachingGate><TicketAdmin /></CoachingGate>; }
