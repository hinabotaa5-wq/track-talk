"use client";

import { CircleCheck } from "lucide-react";
import Link from "next/link";
import { useEffect, useRef, useState } from "react";
import { useAuth } from "@/components/auth-provider";
import { Button } from "@/components/ui/button";
import { ApiRequestError, apiFetch } from "@/lib/api";
import type { ProductKind } from "@/lib/home-sort";
import { productKindLabels } from "@/lib/product";
import { useActionLock } from "@/lib/use-action-lock";

const input = "mt-1 w-full rounded-lg border border-white/20 bg-white/10 px-3 py-2 text-sm text-white";

export function ProductRequest({ kind }: { kind: ProductKind }) {
  const { session, token } = useAuth();
  const [open, setOpen] = useState(false);
  const [brand, setBrand] = useState("");
  const [name, setName] = useState("");
  const [url, setUrl] = useState("");
  const [error, setError] = useState("");
  const [done, setDone] = useState(false);
  const { pending, acquire, release } = useActionLock();
  const doneRef = useRef<HTMLDivElement>(null);

  // フォームが閉じて高さが縮むので、完了の表示が画面の外に出ないようにする。
  useEffect(() => {
    if (done) doneRef.current?.scrollIntoView({ block: "nearest", behavior: "smooth" });
  }, [done]);

  async function submit(e: React.FormEvent) {
    e.preventDefault();
    if (!acquire()) return;
    setError("");
    try {
      await apiFetch("/v1/product-requests", {
        method: "POST",
        token,
        body: JSON.stringify({ kind, brand, name, url }),
      });
      setBrand("");
      setName("");
      setUrl("");
      setDone(true);
      setOpen(false);
    } catch (err) {
      setError(err instanceof ApiRequestError ? err.message : "送信できませんでした");
    } finally {
      release();
    }
  }

  return (
    <section className="mt-6 space-y-3 rounded-xl border border-white/20 glass bg-white/10 p-3.5 text-sm">
      <p className="font-medium">探している{productKindLabels[kind]}がありませんか？</p>
      {done ? (
        <div ref={doneRef} role="status" className="flex scroll-mb-24 items-start gap-2 rounded-lg border border-emerald-500/60 bg-emerald-950/50 p-3">
          <CircleCheck className="mt-0.5 size-5 shrink-0 text-emerald-300" aria-hidden />
          <div>
            <p className="font-semibold text-emerald-200">リクエストしました</p>
            <p className="mt-0.5 text-emerald-100">運営が確認して追加します。</p>
          </div>
        </div>
      ) : null}
      {!session ? (
        <p className="text-slate-200">
          リクエストするには<Link href="/login" className="text-blue-300 underline">ログイン</Link>してください。
        </p>
      ) : !open ? (
        <Button type="button" variant="secondary" onClick={() => { setDone(false); setOpen(true); }}>
          {done ? "ほかの商品もリクエストする" : "追加をリクエストする"}
        </Button>
      ) : (
        <form onSubmit={submit} className="space-y-3">
          <label className="block">
            ブランド
            <input className={input} value={brand} maxLength={60} onChange={(e) => setBrand(e.target.value)} required />
          </label>
          <label className="block">
            商品名
            <input className={input} value={name} maxLength={120} onChange={(e) => setName(e.target.value)} required />
          </label>
          <label className="block">
            公式ページURL（https、任意）
            <input className={input} value={url} maxLength={500} inputMode="url" onChange={(e) => setUrl(e.target.value)} />
          </label>
          {error ? <p className="text-red-300">{error}</p> : null}
          <div className="flex gap-2">
            <Button type="submit" disabled={pending}>{pending ? "送信中..." : "送信"}</Button>
            <Button type="button" variant="secondary" onClick={() => setOpen(false)}>キャンセル</Button>
          </div>
        </form>
      )}
    </section>
  );
}
