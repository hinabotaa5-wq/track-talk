"use client";
import { L } from "@/lib/i18n";

import { SellerNote } from "@/components/seller-note";
import { MinorPurchaseDeclaration } from "@/components/minor-purchase-declaration";
import { PurchaseConfirm } from "@/components/purchase-confirm";
import { TicketBalance } from "@/components/ticket-balance";
import { PageLoading } from "@/components/page-loading";

import { useAuth } from "@/components/auth-provider";
import { Button } from "@/components/ui/button";
import { apiFetch, ApiRequestError } from "@/lib/api";
import { ticketReturnPath, ticketStatusLabels, type TicketProduct, type TicketSummary } from "@/lib/coaching-tickets";
import { formatDateTime, formatYen } from "@/lib/format";
import { useMinorDeclaration } from "@/lib/minor-purchase";
import { ticketPackConfirmRows, ticketRefundExceptions } from "@/lib/purchase-confirm";
import { reviewPriceYen, ticketValidityMonths } from "@/lib/review-pricing";
import { useActionLock } from "@/lib/use-action-lock";
import { useRouter, useSearchParams } from "next/navigation";
import { Suspense, useCallback, useEffect, useState } from "react";

function TicketsPage() {
  const { session, token, loading } = useAuth();
  const router = useRouter();
  const params = useSearchParams();
  const purchaseID = params.get("purchase");
  const returnPath = ticketReturnPath(params.get("return_to"));
  const [summary, setSummary] = useState<TicketSummary | null>(null);
  const [error, setError] = useState("");
  const [message, setMessage] = useState("");
  const { pending, acquire, release } = useActionLock();
  // 支払い画面へ進む前の最終確認に出している商品。
  const [confirming, setConfirming] = useState<TicketProduct | null>(null);
  // 全員の購入前チェック（単発と同じ。毎回未チェックから始める）。
  const declaration = useMinorDeclaration();
  const needsDeclaration = true;

  const load = useCallback(async () => {
    const data = await apiFetch<TicketSummary>("/v1/me/coaching/tickets", { token });
    setSummary(data);
  }, [token]);

  useEffect(() => {
    if (!loading && !session) router.replace(`/login?next=${encodeURIComponent("/me/coaching/tickets" + (params.toString() ? `?${params}` : ""))}`);
  }, [loading, session, router, params]);

  useEffect(() => {
    if (!token) return;
    let cancelled = false;
    apiFetch<TicketSummary>("/v1/me/coaching/tickets", { token }).then((data) => {
      if (!cancelled) setSummary(data);
    }).catch((err) => { if (!cancelled) setError(err instanceof ApiRequestError ? err.message : L("読み込めませんでした")); });
    return () => { cancelled = true; };
  }, [token]);

  // 決済画面から戻っただけでは付与済みとみなさず、サーバーの確定を待つ。
  useEffect(() => {
    if (!token || !purchaseID) return;
    let cancelled = false;
    let timer: ReturnType<typeof setTimeout> | undefined;
    let attempts = 0;
    async function check() {
      try {
        const purchase = await apiFetch<{ status: string; return_path: string }>(`/v1/coaching/tickets/purchases/${encodeURIComponent(purchaseID!)}`, { token });
        if (cancelled) return;
        if (purchase.status !== "pending") {
          await load();
          if (cancelled) return;
          setMessage(L("購入内容を反映しました"));
          const target = ticketReturnPath(purchase.return_path);
          router.replace(target);
          return;
        }
        setMessage(L("お支払いの確認中です。確認が済みしだい元の画面に戻ります。"));
        if (++attempts < 30) timer = setTimeout(() => void check(), 2000);
        else setMessage(L("確認に時間がかかっています。少し待ってから再読み込みしてください。追加購入は必要ありません。"));
      } catch (err) {
        if (!cancelled) setError(err instanceof ApiRequestError ? err.message : L("購入状況を確認できませんでした。画面を再読み込みしてください。"));
      }
    }
    void check();
    return () => { cancelled = true; if (timer) clearTimeout(timer); };
  }, [token, purchaseID, load, router]);

  async function buy(productID: string) {
    if (!session || !summary) return;
    if (needsDeclaration && !declaration.checked) {
      setError(L("購入前の注意事項を確認して、チェックを入れてください。"));
      return;
    }
    if (!acquire()) return;
    setError("");
    const storageKey = `tracktalk-tickets:${session.user.id}:${productID}:${returnPath}`;
    try {
      let key = sessionStorage.getItem(storageKey);
      if (!key || summary.purchases?.some((p) => p.request_key === key && p.status !== "pending")) {
        key = crypto.randomUUID();
        sessionStorage.setItem(storageKey, key);
      }
      const result = await apiFetch<{ checkout_url?: string; paid?: boolean; return_path?: string }>("/v1/coaching/tickets/checkout", {
        method: "POST", token, body: JSON.stringify({ product_id: productID, request_key: key, return_path: returnPath, ...declaration.body(needsDeclaration) }),
      });
      if (result.paid) {
        sessionStorage.removeItem(storageKey);
        await load();
        router.replace(ticketReturnPath(result.return_path));
      } else if (result.checkout_url) window.location.assign(result.checkout_url);
    } catch (err) {
      if (err instanceof ApiRequestError && err.code === "ticket_checkout_expired") sessionStorage.removeItem(storageKey);
      setError(err instanceof ApiRequestError ? err.message : L("購入画面を開けませんでした。同じボタンで再試行できます。"));
    } finally { release(); }
  }

  if (loading || !session) return <PageLoading screen label={L("読み込み中...")} />;
  return <div className="mx-auto max-w-md space-y-5">
    <h1 className="text-2xl font-bold">{L("コーチングチケット")}</h1>
    <SellerNote />
    {message ? <p role="status" className="text-sm text-blue-200">{message}</p> : null}
    {error ? <p role="alert" className="text-sm text-red-400">{error}</p> : null}
    <Button type="button" disabled={pending} onClick={() => void load().catch(() => setError(L("読み込めませんでした")))}>{L("チケット情報を再読み込み")}</Button>
    {summary ? <>
      <TicketBalance summary={summary} />
      {summary.sales_open === false ? <p role="status" className="rounded-xl border border-amber-400/40 bg-amber-500/10 p-3 text-sm text-amber-100">{summary.purchase_block_reason}</p> : null}
      {confirming ? (
        <PurchaseConfirm
          rows={ticketPackConfirmRows({ quantity: confirming.quantity, amountYen: confirming.amount_yen, validityMonths: summary.validity_months ?? ticketValidityMonths })}
          confirmLabel={L("確定してお支払いへ（{v0}）", { v0: formatYen(confirming.amount_yen) })}
          onConfirm={() => void buy(confirming.id)}
          onBack={() => { setConfirming(null); setError(""); }}
          pending={pending}
          confirmDisabled={!declaration.checked}
        >
          {needsDeclaration ? <MinorPurchaseDeclaration checked={declaration.checked} onChange={declaration.setChecked} onShown={declaration.markShown} disabled={pending} /> : null}
        </PurchaseConfirm>
      ) : (summary.products ?? []).map((p) => {
        const single = summary.single_price_yen ?? reviewPriceYen;
        return <div key={p.id} className="space-y-2 rounded-xl border border-white/20 p-4">
          <p className="font-semibold">{p.title}</p>
          <p className="text-lg font-bold">{L("{v0}（税込）", { v0: formatYen(p.amount_yen) })}</p>
          {p.quantity > 1 ? <p className="text-sm text-slate-200">{L("1回あたり約{v0}。1回ずつ払うと{v1}×{q}回で{v2}です。", { v0: formatYen(Math.round(p.amount_yen / p.quantity)), v1: formatYen(single), q: p.quantity, v2: formatYen(single * p.quantity) })}</p> : null}
          <ul className="list-disc space-y-1 pl-5 text-xs text-slate-200">
            <li>{L("{q}枚をまとめて先に支払います。1枚で動画添削1回を申し込めます。", { q: p.quantity })}</li>
            <li>{L("有効期限は決済が完了した日時から{m}か月です。過ぎると未使用の分は失効し、延長はしません。", { m: summary.validity_months ?? ticketValidityMonths })}</li>
            <li>{L("購入者の都合による払戻しは原則として行いません。次の場合は払戻しをします。")}
              <ul className="list-disc space-y-1 pl-5 pt-1">
                {ticketRefundExceptions().map((e) => <li key={e}>{e}</li>)}
              </ul>
            </li>
            <li>{L("購入した本人だけが使えます。譲ることはできません。")}</li>
          </ul>
          <Button type="button" className="w-full" disabled={pending || !summary.can_purchase || !!purchaseID} onClick={() => { setError(""); setConfirming(p); }}>{L("購入内容を確認する")}</Button>
        </div>;
      })}
      <p className="text-xs text-slate-300">{L("動画添削1回にチケット1枚を使います。依頼時に確保し、返却時に使用済みになります。期限までに返却がなければ戻ります。")}</p>
      {(summary.purchases ?? []).filter((p) => p.status !== "pending").length > 0 ? <section className="space-y-2">
        <h2 className="text-sm font-bold">{L("購入したチケット")}</h2>
        <ul className="space-y-2">{(summary.purchases ?? []).filter((p) => p.status !== "pending").map((p) => <li key={p.id} className="rounded-xl border border-white/15 p-3 text-xs text-slate-200">
          <p className="font-semibold text-slate-100">{L("{q}枚 {v0} · {status}", { q: p.quantity, v0: formatYen(p.amount_yen), status: ticketStatusLabels[p.status] })}</p>
          <p>{L("未使用 {a}枚 · 確保中 {r}枚 · 使用済み {c}枚 · 失効 {e}枚", { a: p.available, r: p.reserved, c: p.consumed ?? 0, e: p.expired ?? 0 })}</p>
          <p>{p.expires_at ? L("有効期限：{v0}", { v0: formatDateTime(p.expires_at) }) : L("有効期限：なし（以前に購入した分）")}</p>
        </li>)}</ul>
      </section> : null}
    </> : null}
  </div>;
}

export default function Page() { return <Suspense fallback={<PageLoading screen label={L("読み込み中...")} />}><TicketsPage /></Suspense>; }
