"use client";

import { PageLoading } from "@/components/page-loading";

import { useAuth } from "@/components/auth-provider";
import { CoachingGate } from "@/components/coaching-gate";
import { useConfirm } from "@/components/confirm-dialog";
import { Button } from "@/components/ui/button";
import { apiFetch, ApiRequestError } from "@/lib/api";
import { ticketStatusLabels, type TicketPurchase } from "@/lib/coaching-tickets";
import { formatDateTime, formatYen } from "@/lib/format";
import { reviewPriceYen } from "@/lib/review-pricing";
import { useActionLock } from "@/lib/use-action-lock";
import Link from "next/link";
import { useCallback, useEffect, useState } from "react";

type Data = {
  purchases: (TicketPurchase & {
    nickname: string | null; refund_pending: boolean;
    wording_version: string | null; expired: number; expires_at: string | null;
  })[] | null;
  balance_issues: string[] | null;
  pending_refunds: { id: string; purchase_id: string; amount_yen: number; quantity: number }[] | null;
  // Stripe の外で返す分がある返金・Stripe が断って運営者の対応待ちの返金（本体 #572）。
  attention_refunds?: AttentionRefund[] | null;
  single_price_yen: number;
};

type AttentionRefund = {
  id: string; purchase_id: string; quantity: number; amount_yen: number; status: string;
  manual_yen: number; manual_method: string | null; manual_paid_on: string | null; manual_completed_at: string | null;
};

// 今日の日付（日本時間）を YYYY-MM-DD で。
function todayJst(): string {
  return new Date(Date.now() + 9 * 3600 * 1000).toISOString().slice(0, 10);
}

type ReferralRow = {
  code: string;
  owner_id: string;
  nickname: string | null;
  buyers: string[];
  granted_at: string | null;
  canceled_at: string | null;
  qualifying_refund_at: string | null;
};

function ReferralOverview({ token }: { token: string | null }) {
  const [code, setCode] = useState("");
  const [rows, setRows] = useState<ReferralRow[] | null>(null);
  const [error, setError] = useState("");
  const { pending, acquire, release } = useActionLock();

  async function search(value: string) {
    if (!acquire()) return;
    setError("");
    try {
      const result = await apiFetch<{ referrals: ReferralRow[] | null }>(
        `/v1/admin/coaching/referrals?code=${encodeURIComponent(value)}`, { token },
      );
      setRows(result.referrals ?? []);
    } catch (err) {
      setRows(null);
      setError(err instanceof ApiRequestError ? err.message : "紹介情報を読み込めませんでした");
    } finally {
      release();
    }
  }

  return <section className="app-card space-y-2 rounded-xl p-3">
    <h2 className="font-semibold">紹介特典</h2>
    <form className="flex flex-wrap items-end gap-2" onSubmit={(event) => { event.preventDefault(); void search(code); }}>
      <label className="text-sm">紹介ID
        <input className="block rounded-lg border border-slate-500 bg-transparent p-2" value={code} maxLength={32}
          onChange={(event) => setCode(event.target.value)} />
      </label>
      <Button type="submit" disabled={pending || !code.trim()}>検索</Button>
      <Button type="button" variant="secondary" disabled={pending} onClick={() => void search("")}>付与履歴</Button>
    </form>
    {error ? <p role="alert">{error}</p> : null}
    {rows?.length === 0 ? <p>該当する記録はありません</p> : null}
    {rows?.map((row) => <div className="space-y-1 break-all border-t border-slate-500/40 pt-2 text-sm"
      key={`${row.owner_id}:${row.granted_at}`}>
      <p>{row.code}　{row.nickname ?? row.owner_id}</p>
      <p>購入者ID {row.buyers.join("、") || "なし"}</p>
      <p>{row.granted_at ? `付与 ${formatDateTime(row.granted_at)}` : "未付与"}
        {row.canceled_at ? ` ／ 取消し ${formatDateTime(row.canceled_at)}` : row.qualifying_refund_at ? " ／ 紹介購入の返金あり" : ""}
      </p>
    </div>)}
  </section>;
}

function TicketAdmin() {
  const { token } = useAuth();
  const [data, setData] = useState<Data | null>(null);
  const [error, setError] = useState("");
  const [message, setMessage] = useState("");
  // 失効後の返金で返す枚数（購入ごと。本体 #572 項目7：運営者・コーチの都合の確認は申出ベース）。
  const [postExpiryQty, setPostExpiryQty] = useState<Record<string, string>>({});
  // 失効後の返金は1枚あたり単発価格。API の応答がなければ単発の定価を使う。
  const singlePriceYen = data?.single_price_yen ?? reviewPriceYen;
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

  // Stripe の外（振込など）で返した分の支払いを記録する（本体 #572、運営者決定 2026-11-12）。
  const [manualForm, setManualForm] = useState<Record<string, { method: string; paidOn: string }>>({});
  async function completeManual(r: AttentionRefund) {
    const form = manualForm[r.id] ?? { method: "銀行振込", paidOn: todayJst() };
    const yen = r.status === "needs_action" ? r.amount_yen : r.manual_yen;
    if (!(await confirmDialog({ title: "Stripe の外で返した分を記録しますか？", body: `${formatYen(yen)}を${form.method}で${form.paidOn}に支払った記録を残します。実際に支払った後にだけ記録してください。`, confirmLabel: "記録する" }))) return;
    if (!acquire()) return;
    setError(""); setMessage("");
    try {
      const result = await apiFetch<{ message: string }>(`/v1/admin/coaching/tickets/refunds/${r.id}/manual-complete`, { method: "POST", token, body: JSON.stringify({ method: form.method, paid_on: form.paidOn }) });
      setMessage(result.message);
      await load();
    } catch (err) { setError(err instanceof ApiRequestError ? err.message : "記録できませんでした"); }
    finally { release(); }
  }

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

  // 失効後の返金（本体 #572 項目7）。期限内に受けられなかったことを確認できた失効済みチケットを、
  // 1枚あたり単発価格に相当する額で元の決済へ返す。自動判定はしない。
  async function refundPostExpiry(p: TicketPurchase & { expired: number }, singlePriceYen: number) {
    const quantity = Number(postExpiryQty[p.id] ?? "");
    if (!Number.isInteger(quantity) || quantity < 1 || quantity > p.expired) {
      setError(`返金する枚数は1〜${p.expired}で入力してください`);
      return;
    }
    if (!(await confirmDialog({ title: "失効後の返金をしますか？", body: `失効済みの${quantity}枚を1枚${formatYen(singlePriceYen)}（計${formatYen(singlePriceYen * quantity)}）で返金します。元の決済額を超える分は Stripe では返せないため、振込などで返して後から記録します。運営者・コーチの都合で期限内に受けられなかったことを確認済みの場合にだけ実行してください。`, confirmLabel: "返金する" }))) return;
    if (!acquire()) return;
    setError(""); setMessage("");
    try {
      const result = await apiFetch<{ message: string }>(`/v1/admin/coaching/tickets/${p.id}/refund`, { method: "POST", token, body: JSON.stringify({ post_expiry: true, quantity }) });
      setMessage(result.message);
      await load();
    } catch (err) { setError(err instanceof ApiRequestError ? err.message : "返金を受け付けられませんでした"); }
    finally { release(); }
  }

  return <div className="mx-auto max-w-2xl space-y-4">
    <h1 className="text-xl font-bold">チケット購入・返金管理</h1>
    <ReferralOverview token={token} />
    <Link href="/admin" className="text-sm text-blue-300 underline">運営画面に戻る</Link>
    {error ? <p role="alert" className="text-sm text-red-400">{error}</p> : null}
    {message ? <p role="status" className="text-sm text-blue-200">{message}</p> : null}
    <Button type="button" disabled={pending} onClick={() => void load().catch(() => setError("読み込めませんでした"))}>再読み込み</Button>
    {data ? <>
      <p className={(data.balance_issues ?? []).length ? "text-amber-300" : "text-slate-200"}>残高と台帳の不一致：{data.balance_issues?.length ?? 0}件</p>
      {(data.balance_issues ?? []).map((id) => <p key={id} className="break-all text-xs text-amber-300">要確認の購入：{id}</p>)}
      <p className="text-sm">処理中の返金：{data.pending_refunds?.length ?? 0}件（定期処理で照合・再実行します）</p>
      {(data.attention_refunds ?? []).length ? <section aria-label="Stripe の外で返す返金" className="space-y-2">
        <h2 className="font-semibold">Stripe の外で返す返金</h2>
        <p className="text-xs text-slate-300">1つの決済には元の決済額までしか Stripe で返金できません。超えた分は振込などで返し、支払った後に記録してください。</p>
        <ul className="space-y-2">{(data.attention_refunds ?? []).map((r) => {
          const form = manualForm[r.id] ?? { method: "銀行振込", paidOn: todayJst() };
          const open = r.status === "needs_action" || (r.manual_yen > 0 && !r.manual_completed_at);
          return <li key={r.id} className="space-y-1 rounded-xl border border-amber-400/40 p-3 text-sm">
            <p>{r.quantity}枚・合計 {formatYen(r.amount_yen)}（Stripe {formatYen(r.status === "needs_action" ? 0 : r.amount_yen - r.manual_yen)}／Stripe 外 {formatYen(r.status === "needs_action" ? r.amount_yen : r.manual_yen)}）</p>
            <p className="text-xs">{r.status === "needs_action" ? "Stripe が返金を断りました。全額を Stripe の外で返してください" : r.manual_completed_at ? `支払い済み：${r.manual_method ?? ""}・${r.manual_paid_on ?? ""}` : "Stripe 外の分は支払い待ちです"}</p>
            <p className="break-all text-xs text-slate-300">購入 {r.purchase_id}</p>
            {open ? <div className="flex flex-wrap items-center gap-2">
              <label className="text-xs">支払方法 <input value={form.method} maxLength={40} disabled={pending}
                onChange={(e) => setManualForm((prev) => ({ ...prev, [r.id]: { ...form, method: e.target.value } }))}
                className="w-28 rounded border border-white/30 bg-black/30 px-2 py-1 text-sm" /></label>
              <label className="text-xs">支払日 <input type="date" value={form.paidOn} disabled={pending}
                onChange={(e) => setManualForm((prev) => ({ ...prev, [r.id]: { ...form, paidOn: e.target.value } }))}
                className="rounded border border-white/30 bg-black/30 px-2 py-1 text-sm" /></label>
              <Button type="button" disabled={pending} onClick={() => void completeManual(r)}>支払いを記録する</Button>
            </div> : null}
          </li>;
        })}</ul>
      </section> : null}
      <ul className="space-y-3">{(data.purchases ?? []).map((p) => <li key={p.id} className="space-y-2 rounded-xl border border-white/20 p-3 text-sm">
        <p className="font-semibold">{p.nickname ?? "退会した利用者"}：{p.quantity}枚・{formatYen(p.amount_yen)}・{ticketStatusLabels[p.status]}</p>
        <p>未使用 {p.available}枚／確保中 {p.reserved}枚／失効 {p.expired}枚／返金済み {formatYen(p.refunded_yen)}</p>
        <p className="break-all text-xs text-slate-300">{p.id} · {formatDateTime(p.created_at)}{p.expires_at ? ` · 有効期限 ${formatDateTime(p.expires_at)}` : ""}{p.wording_version ? ` · 文面版 ${p.wording_version}` : ""}</p>
        <Button type="button" disabled={pending || (!p.refund_pending && p.available === 0)} onClick={() => void refund(p)}>
          {p.refund_pending ? "返金状況を照合・再試行" : "未使用分を返金する"}
        </Button>
        {p.expired > 0 ? <div className="space-y-2 rounded-lg border border-amber-400/40 bg-amber-500/10 p-2">
          <p className="text-xs text-amber-100">失効済み {p.expired}枚：運営者・コーチの都合で期限内に受けられなかったことを確認できた分だけ、1枚{formatYen(singlePriceYen)}で失効後に返金できます。</p>
          <div className="flex items-center gap-2">
            <label className="text-xs">枚数 <input type="number" min={1} max={p.expired} value={postExpiryQty[p.id] ?? ""} disabled={pending}
              onChange={(e) => setPostExpiryQty((prev) => ({ ...prev, [p.id]: e.target.value }))}
              className="w-20 rounded border border-white/30 bg-black/30 px-2 py-1 text-sm" /></label>
            <Button type="button" disabled={pending} onClick={() => void refundPostExpiry(p, singlePriceYen)}>失効後に返金する</Button>
          </div>
        </div> : null}
      </li>)}</ul>
    </> : <PageLoading label="読み込み中..." />}
  </div>;
}

export default function Page() { return <CoachingGate><TicketAdmin /></CoachingGate>; }
