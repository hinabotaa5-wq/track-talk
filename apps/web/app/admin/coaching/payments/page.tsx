"use client";

import { useAuth } from "@/components/auth-provider";
import { CoachingGate } from "@/components/coaching-gate";
import { useConfirm } from "@/components/confirm-dialog";
import { PageLoading } from "@/components/page-loading";
import { Button } from "@/components/ui/button";
import { apiFetch, ApiRequestError } from "@/lib/api";
import { formatDateTime, formatYen } from "@/lib/format";
import { useActionLock } from "@/lib/use-action-lock";
import Link from "next/link";
import { useCallback, useEffect, useState } from "react";

type Payment = { id: string; source: "review" | "consultation"; amount_yen: number; status: string; created_at: string; minor?: boolean };
// 18歳未満の本人が自分で払った動画添削・追加質問の返金（自動の返金を含む）。admin_* は管理画面から返金した分。
type MinorRefundCounts = { review_count: number; review_amount_yen: number; consultation_count: number; consultation_amount_yen: number; admin_count: number; admin_amount_yen: number };
type Refund = { id: string; payment_id: string; source: string; amount_yen: number; reason: string; admin_id: string | null; origin: string; status: string; failure: string | null; stripe_refund_id: string | null; created_at: string };
type Adjustment = { id: string; coach_id: string | null; refund_id: string; amount_yen: number; applied_yen: number; remaining_yen: number; canceled_yen?: number; compensation?: { amount_yen: number; earning_id: string; payout_id: string | null } | null };
type Data = { payments: Payment[]; refunds: Refund[]; adjustments: Adjustment[]; has_more?: boolean; page?: number; minor_refunds?: { month: MinorRefundCounts; total: MinorRefundCounts } };

function minorRefundLine(label: string, c: MinorRefundCounts) {
 return `${label}：フォーム添削 ${c.review_count}件 ${formatYen(c.review_amount_yen)} · 追加質問 ${c.consultation_count}件 ${formatYen(c.consultation_amount_yen)}（うち管理画面から ${c.admin_count}件 ${formatYen(c.admin_amount_yen)}）`;
}
const statusLabels: Record<string, string> = { captured: "確定済み", paid: "支払い済み", refund_pending: "返金処理中", refunded: "返金済み", pending: "処理中", succeeded: "完了", failed: "失敗" };

function PaymentAdmin() {
 const { token } = useAuth();
 const [data, setData] = useState<Data | null>(null);
 const [error, setError] = useState("");
 const [message, setMessage] = useState("");
 const [reasons, setReasons] = useState<Record<string, string>>({});
 const [page, setPage] = useState(1);
 const [filters, setFilters] = useState({ coach: "", refund: "", unapplied: false });
 const [draft, setDraft] = useState(filters);
 const params = new URLSearchParams();
 if (page > 1) params.set("page", String(page));
 if (filters.coach) params.set("coach_id", filters.coach);
 if (filters.refund) params.set("refund_id", filters.refund);
 if (filters.unapplied) params.set("unapplied", "true");
 const endpoint = "/v1/admin/coaching/payments" + (params.size ? `?${params}` : "");
 const { pending, acquire, release } = useActionLock();
 const confirmDialog = useConfirm();
 const load = useCallback(async () => setData(await apiFetch<Data>(endpoint, { token })), [token, endpoint]);
 useEffect(() => {
  if (!token) return;
  let cancelled = false;
  apiFetch<Data>(endpoint, { token }).then(d => { if (!cancelled) setData(d); }).catch(err => { if (!cancelled) setError(err instanceof ApiRequestError ? err.message : "読み込めませんでした"); });
  return () => { cancelled = true; };
 }, [token, endpoint]);
 async function refund(p: Payment) {
  const key = `${p.source}:${p.id}`;
  const reason = (reasons[key] ?? "").trim();
  if (!reason) { setError("返金理由を入力してください"); return; }
  if (!(await confirmDialog({ title: "全額返金しますか？", body: `${formatYen(p.amount_yen)}を返金します。理由：${reason}`, confirmLabel: "全額返金する" }))) return;
  if (!acquire()) return;
  setError(""); setMessage("");
  try {
   const result = await apiFetch<{ message: string }>(`/v1/admin/coaching/payments/${p.id}/refund`, { method: "POST", token, body: JSON.stringify({ source: p.source, reason }) });
   setMessage(result.message); await load();
  } catch (err) { setError(err instanceof ApiRequestError ? err.message : "返金を受け付けられませんでした"); }
  finally { release(); }
 }
 return <div className="mx-auto max-w-3xl space-y-5">
  <h1 className="text-xl font-bold">フォーム添削・追加質問の返金管理</h1>
  <Link href="/admin" className="text-sm text-blue-300 underline">運営画面に戻る</Link>
  <p className="text-sm">確定済みのカード支払いを全額返金します。送金済みのコーチ報酬は今後の報酬と相殺します。コーチ・発生元返金で絞り込み、古い記録までページを送って確認できます。</p>
  {error ? <p role="alert" className="text-red-400">{error}</p> : null}
  {message ? <p role="status">{message}</p> : null}
  <Button disabled={pending} onClick={() => void load().catch(() => setError("読み込めませんでした"))}>再読み込み</Button>
  <form className="space-y-2 rounded border border-white/20 p-3 text-sm" onSubmit={e => { e.preventDefault(); setPage(1); setFilters({ ...draft, coach: draft.coach.trim(), refund: draft.refund.trim() }); }}>
   <label className="block">コーチID<input className="ml-2 rounded border border-white/30 bg-transparent p-1" value={draft.coach} onChange={e => setDraft(d => ({ ...d, coach: e.target.value }))} /></label>
   <label className="block">発生元の返金ID<input className="ml-2 rounded border border-white/30 bg-transparent p-1" value={draft.refund} onChange={e => setDraft(d => ({ ...d, refund: e.target.value }))} /></label>
   <label className="block"><input type="checkbox" checked={draft.unapplied} onChange={e => setDraft(d => ({ ...d, unapplied: e.target.checked }))} /> 未適用の相殺だけを表示</label>
   <Button type="submit" disabled={pending}>絞り込む</Button>
   <Button type="button" disabled={pending} onClick={() => { const f = { coach: "", refund: "", unapplied: false }; setDraft(f); setFilters(f); setPage(1); }}>条件を解除</Button>
  </form>
  {data ? <>
   {data.minor_refunds ? <section className="space-y-1 rounded border border-white/20 p-3 text-sm" aria-label="未成年の購入の返金">
    <h2 className="font-bold">未成年の購入の返金</h2>
    <p>{minorRefundLine("今月", data.minor_refunds.month)}</p>
    <p>{minorRefundLine("累計", data.minor_refunds.total)}</p>
    <p className="text-xs text-slate-300">18歳未満の本人が自分で払ったフォーム添削・追加質問のうち、返金が済んだもの。期限切れなどの自動の返金も含みます。</p>
   </section> : null}
   <h2 className="font-bold">支払い</h2>
   <ul className="space-y-3">{data.payments.map(p => { const key = `${p.source}:${p.id}`; return <li key={key} className="space-y-2 rounded-xl border border-white/20 p-3 text-sm">
    <p>{p.source === "review" ? "フォーム添削" : "追加質問"} · {formatYen(p.amount_yen)} · {statusLabels[p.status] ?? p.status}{p.minor ? " · 未成年の購入" : ""}</p>
    <p className="break-all text-xs">{p.id} · {formatDateTime(p.created_at)}</p>
    {p.status === "captured" || p.status === "paid" || p.status === "refund_pending" ? <>
     <label className="block">返金理由<input className="mt-1 block w-full rounded border border-white/30 bg-transparent p-2" maxLength={1000} value={reasons[key] ?? ""} onChange={e => setReasons(old => ({ ...old, [key]: e.target.value }))} /></label>
     <Button disabled={pending || !(reasons[key] ?? "").trim()} onClick={() => void refund(p)}>{p.status === "refund_pending" ? "返金状況を照合" : "全額返金する"}</Button>
    </> : null}
   </li>; })}</ul>
   <h2 className="font-bold">コーチ報酬の相殺台帳</h2>
   {data.adjustments.length === 0 ? <p className="text-sm">相殺の記録はありません</p> : null}
   <ul className="space-y-2">{data.adjustments.map(a => <li key={a.id} className="rounded border border-white/20 p-3 text-sm">
    <p className="break-all">コーチ：{a.coach_id ?? "退会済み"}</p>
    <p>相殺額 {formatYen(a.amount_yen)} · 適用済み {formatYen(a.applied_yen)} · 取消 {formatYen(a.canceled_yen ?? 0)} · 未適用 {formatYen(a.remaining_yen)}</p>
    {a.compensation ? <p>支払う補償額 {formatYen(a.compensation.amount_yen)} · {a.compensation.payout_id ? "送金に組込み済み" : "月次送金待ち"}</p> : null}
    <p className="break-all text-xs">発生元の返金：{a.refund_id}</p>
    <Button type="button" disabled={pending} onClick={() => { const f = { ...filters, refund: a.refund_id, unapplied: false }; setDraft(f); setFilters(f); setPage(1); }}>発生元の返金・監査を確認</Button>
   </li>)}</ul>
   <h2 className="font-bold">返金の監査記録</h2>
   <ul className="space-y-2">{data.refunds.map(f => <li key={f.id} className="space-y-1 rounded border border-white/20 p-3 text-sm">
    <p>{formatYen(f.amount_yen)} · {statusLabels[f.status] ?? f.status} · {formatDateTime(f.created_at)}</p>
    <p className="break-all">返金：{f.id} · 支払い：{f.payment_id}</p>
    <p>理由：{f.reason}</p><p className="break-all">実行者：{f.origin === "stripe" ? "Stripe ダッシュボード" : f.admin_id ?? "退会済み管理者"}</p>
    {f.stripe_refund_id ? <p className="break-all">Stripe返金：{f.stripe_refund_id}</p> : null}
    {f.failure ? <p role="alert">{f.failure}</p> : null}
   </li>)}</ul>
   <nav aria-label="返金一覧のページ" className="flex items-center gap-3">
    <Button type="button" disabled={pending || page === 1} onClick={() => setPage(p => p - 1)}>前のページ</Button>
    <span>{data.page ?? page}ページ</span>
    <Button type="button" disabled={pending || !data.has_more} onClick={() => setPage(p => p + 1)}>次のページ</Button>
   </nav>
  </> : <PageLoading label="読み込み中..." />}
 </div>;
}
export default function Page() { return <CoachingGate><PaymentAdmin /></CoachingGate>; }
