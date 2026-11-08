"use client";

import { SellerNote } from "@/components/seller-note";
import { PageLoading } from "@/components/page-loading";

import { useAuth } from "@/components/auth-provider";
import { Button } from "@/components/ui/button";
import { apiFetch, ApiRequestError } from "@/lib/api";
import { ticketReturnPath, type TicketSummary } from "@/lib/coaching-tickets";
import { formatYen } from "@/lib/format";
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
    }).catch((err) => { if (!cancelled) setError(err instanceof ApiRequestError ? err.message : "読み込めませんでした"); });
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
          setMessage("購入内容を反映しました");
          const target = ticketReturnPath(purchase.return_path);
          router.replace(target);
          return;
        }
        setMessage("お支払いの確認中です。確認が済みしだい元の画面に戻ります。");
        if (++attempts < 30) timer = setTimeout(() => void check(), 2000);
        else setMessage("確認に時間がかかっています。少し待ってから再読み込みしてください。追加購入は必要ありません。");
      } catch (err) {
        if (!cancelled) setError(err instanceof ApiRequestError ? err.message : "購入状況を確認できませんでした。画面を再読み込みしてください。");
      }
    }
    void check();
    return () => { cancelled = true; if (timer) clearTimeout(timer); };
  }, [token, purchaseID, load, router]);

  async function buy(productID: string) {
    if (!session || !summary || !acquire()) return;
    setError("");
    const storageKey = `tracktalk-tickets:${session.user.id}:${productID}:${returnPath}`;
    try {
      let key = sessionStorage.getItem(storageKey);
      if (!key || summary.purchases?.some((p) => p.request_key === key && p.status !== "pending")) {
        key = crypto.randomUUID();
        sessionStorage.setItem(storageKey, key);
      }
      const result = await apiFetch<{ checkout_url?: string; paid?: boolean; return_path?: string }>("/v1/coaching/tickets/checkout", {
        method: "POST", token, body: JSON.stringify({ product_id: productID, request_key: key, return_path: returnPath }),
      });
      if (result.paid) {
        sessionStorage.removeItem(storageKey);
        await load();
        router.replace(ticketReturnPath(result.return_path));
      } else if (result.checkout_url) window.location.assign(result.checkout_url);
    } catch (err) {
      if (err instanceof ApiRequestError && err.code === "ticket_checkout_expired") sessionStorage.removeItem(storageKey);
      setError(err instanceof ApiRequestError ? err.message : "購入画面を開けませんでした。同じボタンで再試行できます。");
    } finally { release(); }
  }

  if (loading || !session) return <PageLoading screen label="読み込み中..." />;
  return <div className="mx-auto max-w-md space-y-5">
    <h1 className="text-2xl font-bold">コーチングチケット</h1>
    <SellerNote />
    {message ? <p role="status" className="text-sm text-blue-200">{message}</p> : null}
    {error ? <p role="alert" className="text-sm text-red-400">{error}</p> : null}
    <Button type="button" disabled={pending} onClick={() => void load().catch(() => setError("読み込めませんでした"))}>チケット情報を再読み込み</Button>
    {summary ? <>
      {summary.sales_open === false ? <p role="status" className="rounded-xl border border-amber-400/40 bg-amber-500/10 p-3 text-sm text-amber-100">{summary.purchase_block_reason}</p> : null}
      <p className="text-sm text-slate-200">動画添削は2枚、追加相談は1枚です。依頼時に確保し、回答時に消費します。期限までに回答がなければ戻ります。有効期限はありません。</p>
      <div className="grid grid-cols-2 gap-3">{(summary.products ?? []).map((p) => <div key={p.id} className="space-y-2 rounded-xl border border-white/20 p-3">
        <p className="font-semibold">{p.title}</p><p>{formatYen(p.amount_yen)}</p>
        {p.quantity > 1 ? <p className="text-xs text-slate-300">1枚あたり{formatYen(p.amount_yen / p.quantity)}</p> : null}
        <Button type="button" className="w-full" disabled={pending || !summary.can_purchase || !!purchaseID} onClick={() => void buy(p.id)}>購入する</Button>
      </div>)}</div>
    </> : null}
  </div>;
}

export default function Page() { return <Suspense fallback={<PageLoading screen label="読み込み中..." />}><TicketsPage /></Suspense>; }
