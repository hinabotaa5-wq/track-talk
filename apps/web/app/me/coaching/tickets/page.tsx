"use client";
import Link from "next/link";
import { LoadedReferralCard } from "@/components/coaching-referral";
import { L } from "@/lib/i18n";

import { SellerNote } from "@/components/seller-note";
import { TicketBalance } from "@/components/ticket-balance";
import { PageLoading } from "@/components/page-loading";

import { useAuth } from "@/components/auth-provider";
import { Button } from "@/components/ui/button";
import { apiFetch, ApiRequestError } from "@/lib/api";
import { ticketStatusLabels, type CouponSummary, type TicketSummary } from "@/lib/coaching-tickets";
import { formatDateTime, formatYen } from "@/lib/format";
import { useRouter, useSearchParams } from "next/navigation";
import { Suspense, useCallback, useEffect, useState } from "react";

// チケットの販売は終了した（本体 #1031）。この画面は、発行済みのチケットの残高・履歴と、
// 紹介特典の ¥500 クーポンの保有状況を見るだけにする（購入の導線は置かない）。

// CouponBalance は、¥500 クーポンの枚数といちばん早い失効日。
function CouponBalance({ coupons }: { coupons: CouponSummary }) {
  return (
    <section className="space-y-1 rounded-xl border border-white/20 bg-white/5 p-3 text-sm" aria-label={L("クーポンの残り")}>
      <p className="font-semibold">{L("¥{v0}クーポンの残り {n}枚", { v0: coupons.amount_yen.toLocaleString("ja-JP"), n: coupons.available })}</p>
      {coupons.available > 0 && coupons.next_expires_at ? (
        <p className="text-xs text-slate-200">{L("いちばん早い有効期限：{v0}。過ぎると使えなくなります。", { v0: formatDateTime(coupons.next_expires_at) })}</p>
      ) : null}
      <p className="text-xs text-slate-300">{L("スタンダードの申込みで1回に1枚使えます。チケットとは一緒に使えません。")}</p>
      {coupons.available > 0 ? (
        <Link href="/coaching" className="inline-block text-xs text-blue-300 underline">{L("コーチを選んでクーポンを使う")}</Link>
      ) : (
        <Link href="/referral" className="inline-block text-xs text-blue-300 underline">{L("紹介でクーポンをもらう方法")}</Link>
      )}
    </section>
  );
}

function TicketsPage() {
  const { session, token, loading } = useAuth();
  const router = useRouter();
  const params = useSearchParams();
  // 本体 #1108：残高は「どの利用者の分か」と組で持ち、表示中の利用者と一致するときだけ出す（#1011 の方式に揃える）。
  const userKey = token ? session?.user?.id ?? token : null;
  const [loaded, setLoaded] = useState<{ key: string | null; summary: TicketSummary | null }>({ key: null, summary: null });
  const [errorLoaded, setErrorLoaded] = useState<{ key: string | null; message: string }>({ key: null, message: "" });
  const summary = userKey && loaded.key === userKey ? loaded.summary : null;
  const error = userKey && errorLoaded.key === userKey ? errorLoaded.message : "";

  const load = useCallback(async () => {
    if (!userKey) return;
    const data = await apiFetch<TicketSummary>("/v1/me/coaching/tickets", { token });
    setLoaded({ key: userKey, summary: data });
    setErrorLoaded({ key: userKey, message: "" });
  }, [token, userKey]);

  useEffect(() => {
    if (!loading && !session) router.replace(`/login?next=${encodeURIComponent("/me/coaching/tickets" + (params.toString() ? `?${params}` : ""))}`);
  }, [loading, session, router, params]);

  useEffect(() => {
    if (!token || !userKey) return;
    let alive = true;
    const run = () => {
      void load().catch((err) => {
        if (alive) setErrorLoaded({ key: userKey, message: err instanceof ApiRequestError ? err.message : L("読み込めませんでした") });
      });
    };
    const refresh = () => { if (document.visibilityState === "visible") run(); };
    // 履歴キャッシュ（bfcache）から戻ったときも取り直す（本体 #1011 と同じ契機）。
    const restored = (event: PageTransitionEvent) => { if (event.persisted) run(); };
    run();
    document.addEventListener("visibilitychange", refresh);
    window.addEventListener("focus", refresh);
    window.addEventListener("pageshow", restored);
    return () => {
      alive = false;
      document.removeEventListener("visibilitychange", refresh);
      window.removeEventListener("focus", refresh);
      window.removeEventListener("pageshow", restored);
    };
  }, [token, userKey, load]);

  if (loading || !session) return <PageLoading screen label={L("読み込み中...")} />;
  const purchases = (summary?.purchases ?? []).filter((p) => p.status !== "pending");
  return <div className="mx-auto max-w-md space-y-5">
    <h1 className="text-2xl font-bold">{L("チケット・クーポン")}</h1>
    <SellerNote />
    {error ? <p role="alert" className="text-sm text-red-400">{error}</p> : null}
    <Button type="button" onClick={() => void load().catch(() => setErrorLoaded({ key: userKey, message: L("読み込めませんでした") }))}>{L("チケット情報を再読み込み")}</Button>
    {summary ? <>
      {summary.coupons ? <CouponBalance coupons={summary.coupons} /> : null}
      <TicketBalance summary={summary} />
      <p className="text-sm text-slate-200">{L("チケットの販売は終了しました。発行済みのチケットは有効期限まで利用できます。")}</p>
      {purchases.length > 0 ? <section className="space-y-2">
        <h2 className="text-sm font-bold">{L("購入したチケット")}</h2>
        <ul className="space-y-2">{purchases.map((p) => <li key={p.id} className="rounded-xl border border-white/15 p-3 text-xs text-slate-200">
          <p className="font-semibold text-slate-100">{L("{q}枚 {v0} · {status}", { q: p.quantity, v0: formatYen(p.amount_yen), status: ticketStatusLabels[p.status] })}</p>
          <p>{L("未使用 {a}枚 · 確保中 {r}枚 · 使用済み {c}枚 · 失効 {e}枚", { a: p.available, r: p.reserved, c: p.consumed ?? 0, e: p.expired ?? 0 })}</p>
          <p>{p.expires_at ? L("有効期限：{v0}", { v0: formatDateTime(p.expires_at) }) : L("有効期限：なし（以前に購入した分）")}</p>
        </li>)}</ul>
      </section> : null}
      <LoadedReferralCard token={token} />
    </> : null}
  </div>;
}

export default function Page() { return <Suspense fallback={<PageLoading screen label={L("読み込み中...")} />}><TicketsPage /></Suspense>; }
