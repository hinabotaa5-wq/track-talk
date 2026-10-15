"use client";

import { useAuth } from "@/components/auth-provider";
import { apiFetch } from "@/lib/api";
import { formatDateTime } from "@/lib/format";
import { productKindLabels } from "@/lib/product";
import type { AdminInquiry, AdminReport, Meta, Product } from "@/lib/types";
import { inquiryCategoryLabels } from "@/lib/validators";
import Link from "next/link";
import { useRouter } from "next/navigation";
import { useCallback, useEffect, useState } from "react";

const reasonLabels: Record<AdminReport["reason"], string> = {
  spam: "スパム",
  harassment: "嫌がらせ",
  misinfo: "誤情報",
  other: "その他",
};

const statusLabels: Record<AdminReport["status"], string> = {
  open: "未対応",
  reviewed: "確認済み",
  actioned: "対応済み",
  dismissed: "却下",
};

const perPage = 20;

type Tab = "reports" | "feedback" | "contact" | "requests" | "products";

function useAdminList<T>(path: string, key: "reports" | "inquiries", enabled: boolean, token: string | null) {
  const [items, setItems] = useState<T[]>([]);
  const [total, setTotal] = useState(0);
  const [page, setPage] = useState(1);
  const [error, setError] = useState("");
  const [loadedPage, setLoadedPage] = useState(0);

  useEffect(() => {
    if (!enabled || !token) return;
    let cancelled = false;
    apiFetch<Record<string, unknown> & { meta: Meta }>(`${path}${path.includes("?") ? "&" : "?"}page=${page}&per_page=${perPage}`, { token })
      .then((d) => {
        if (cancelled) return;
        const rows = (d[key] as T[] | undefined) ?? [];
        setItems((current) => (page === 1 ? rows : [...current, ...rows]));
        setTotal(d.meta.total);
        setError("");
        setLoadedPage(page);
      })
      .catch(() => {
        if (cancelled) return;
        setError("取得に失敗しました");
        setLoadedPage(page);
      });
    return () => {
      cancelled = true;
    };
  }, [path, key, enabled, token, page]);

  const loading = enabled && loadedPage < page;
  return { items, total, error, loading, more: items.length < total, loadMore: () => setPage((p) => p + 1) };
}

function ProductLinkEditor({
  product,
  onSave,
  inputClass,
}: {
  product: Product;
  onSave: (next: { product_url: string; discontinued: boolean }) => void;
  inputClass: string;
}) {
  const [url, setUrl] = useState(product.product_url);
  const [discontinued, setDiscontinued] = useState(product.discontinued);
  const dirty = url !== product.product_url || discontinued !== product.discontinued;
  return (
    <div className="flex flex-wrap items-center gap-2">
      <input
        className={`${inputClass} mt-0 min-w-0 flex-1`}
        value={url}
        maxLength={500}
        placeholder="公式ページURL（https）"
        aria-label="公式ページURL"
        onChange={(e) => setUrl(e.target.value)}
      />
      <label className="flex items-center gap-1 text-xs text-slate-200">
        <input type="checkbox" checked={discontinued} onChange={(e) => setDiscontinued(e.target.checked)} />
        旧モデル
      </label>
      <button
        type="button"
        disabled={!dirty}
        onClick={() => onSave({ product_url: url.trim(), discontinued })}
        className="rounded-full bg-blue-600 px-3 py-1 text-xs font-semibold text-white disabled:opacity-40"
      >
        保存
      </button>
    </div>
  );
}

function ProductsAdmin({ token }: { token: string | null }) {
  const [kind, setKind] = useState<"shoe" | "spike">("shoe");
  const [brand, setBrand] = useState("");
  const [name, setName] = useState("");
  const [imageUrl, setImageUrl] = useState("");
  const [productUrl, setProductUrl] = useState("");
  const [products, setProducts] = useState<Product[]>([]);
  const [message, setMessage] = useState("");
  const [pending, setPending] = useState(false);

  const load = useCallback(async () => {
    try {
      const d = await apiFetch<{ products: Product[] }>(`/v1/products?kind=${kind}&per_page=100`);
      setProducts(d.products ?? []);
    } catch {
      setMessage("取得に失敗しました");
    }
  }, [kind]);

  useEffect(() => {
    // eslint-disable-next-line react-hooks/set-state-in-effect
    void load();
  }, [load]);

  async function add(e: React.FormEvent) {
    e.preventDefault();
    if (pending) return;
    setPending(true);
    setMessage("");
    try {
      await apiFetch("/v1/admin/products", {
        method: "POST",
        token,
        body: JSON.stringify({ kind, brand, name, image_url: imageUrl, product_url: productUrl }),
      });
      setBrand("");
      setName("");
      setImageUrl("");
      setProductUrl("");
      setMessage("登録しました");
      await load();
    } catch (err) {
      setMessage(err instanceof Error ? err.message : "登録できませんでした");
    } finally {
      setPending(false);
    }
  }

  async function save(p: Product, next: { product_url: string; discontinued: boolean }) {
    try {
      await apiFetch(`/v1/admin/products/${p.id}`, { method: "PATCH", token, body: JSON.stringify(next) });
      setMessage("更新しました");
      await load();
    } catch (err) {
      setMessage(err instanceof Error ? err.message : "更新できませんでした");
    }
  }

  async function hide(id: string) {
    if (!window.confirm("この商品を非表示にしますか？")) return;
    try {
      await apiFetch(`/v1/admin/products/${id}/hide`, { method: "POST", token });
      await load();
    } catch {
      setMessage("非表示にできませんでした");
    }
  }

  const input = "mt-1 w-full rounded-lg border border-white/20 bg-white/10 px-3 py-2 text-sm text-white";
  return (
    <div className="space-y-4">
      <form onSubmit={add} className="space-y-3 rounded-xl border border-white/20 glass bg-white/10 p-3">
        <div className="flex gap-2">
          {(["shoe", "spike"] as const).map((k) => (
            <button
              key={k}
              type="button"
              onClick={() => setKind(k)}
              className={`rounded-full border px-4 py-1.5 text-sm ${kind === k ? "border-blue-400 bg-blue-500/30 text-white" : "border-white/25 bg-white/10 text-slate-200"}`}
            >
              {productKindLabels[k]}
            </button>
          ))}
        </div>
        <label className="block text-sm">
          ブランド
          <input className={input} value={brand} maxLength={60} onChange={(e) => setBrand(e.target.value)} required />
        </label>
        <label className="block text-sm">
          商品名
          <input className={input} value={name} maxLength={120} onChange={(e) => setName(e.target.value)} required />
        </label>
        <label className="block text-sm">
          画像URL（https、任意）
          <input className={input} value={imageUrl} maxLength={500} onChange={(e) => setImageUrl(e.target.value)} />
        </label>
        <label className="block text-sm">
          公式ページURL（https、任意）
          <input className={input} value={productUrl} maxLength={500} onChange={(e) => setProductUrl(e.target.value)} />
        </label>
        <button type="submit" disabled={pending} className="rounded-full bg-blue-600 px-4 py-2 text-sm font-semibold text-white disabled:opacity-50">
          {pending ? "登録中..." : "商品を登録"}
        </button>
        {message ? <p className="text-sm text-slate-200">{message}</p> : null}
      </form>
      <ul className="space-y-2">
        {products.map((p) => (
          <li key={p.id} className="space-y-2 rounded-xl border border-white/20 glass bg-white/10 p-3 text-sm">
            <div className="flex items-center justify-between gap-3">
              <span>
                {p.brand} {p.name}
                {p.discontinued ? <span className="ml-2 text-xs text-amber-300">旧モデル</span> : null}
                <span className="ml-2 text-xs text-slate-300">レビュー{p.review_count}件</span>
              </span>
              <button type="button" onClick={() => hide(p.id)} className="shrink-0 text-red-300 underline">
                非表示
              </button>
            </div>
            <ProductLinkEditor product={p} onSave={(next) => save(p, next)} inputClass={input} />
          </li>
        ))}
      </ul>
    </div>
  );
}

export default function AdminPage() {
  const { session, profile, token, loading } = useAuth();
  const router = useRouter();
  const [tab, setTab] = useState<Tab>("reports");
  const isAdmin = Boolean(profile?.is_admin);
  const reports = useAdminList<AdminReport>("/v1/admin/reports", "reports", isAdmin, token);
  const feedback = useAdminList<AdminInquiry>("/v1/admin/inquiries?source=feedback", "inquiries", isAdmin, token);
  const contact = useAdminList<AdminInquiry>("/v1/admin/inquiries?source=contact", "inquiries", isAdmin, token);
  const requests = useAdminList<AdminInquiry>("/v1/admin/inquiries?source=product_request", "inquiries", isAdmin, token);

  useEffect(() => {
    if (loading) return;
    if (!session) router.replace("/login");
    else if (!profile?.is_admin) router.replace("/");
  }, [loading, session, profile, router]);

  if (loading || !isAdmin) {
    return <p className="text-sm text-slate-200">読み込み中...</p>;
  }

  const lists = { reports, feedback, contact, requests };
  const list = tab === "products" ? contact : lists[tab];
  const tabs: { id: Tab; label: string; total: number }[] = [
    { id: "reports", label: "通報", total: reports.total },
    { id: "feedback", label: "なんでも書いてね", total: feedback.total },
    { id: "contact", label: "お問い合わせ", total: contact.total },
    { id: "requests", label: "商品リクエスト", total: requests.total },
    { id: "products", label: "商品", total: -1 },
  ];

  return (
    <div className="mx-auto max-w-2xl space-y-4">
      <h1 className="text-2xl font-bold">管理者画面</h1>
      <div role="tablist" className="flex flex-wrap gap-2">
        {tabs.map((t) => (
          <button
            key={t.id}
            type="button"
            role="tab"
            aria-selected={tab === t.id}
            onClick={() => setTab(t.id)}
            className={`rounded-full border px-4 py-2 text-sm font-medium transition active:scale-95 ${
              tab === t.id ? "border-blue-400 bg-blue-500/30 text-white" : "border-white/25 bg-white/10 text-slate-200"
            }`}
          >
            {t.label}{t.total >= 0 ? ` (${t.total})` : ""}
          </button>
        ))}
      </div>
      {tab === "products" ? <ProductsAdmin token={token} /> : null}
      {tab === "products" ? null : list.error ? <p className="text-sm text-red-400">{list.error}</p> : null}
      {tab === "products" ? null : tab === "reports" ? (
        <ul className="space-y-3">
          {reports.items.map((r) => (
            <li key={r.id} className="space-y-2 rounded-xl border border-white/20 glass bg-white/10 p-3">
              <p className="text-xs text-slate-200">
                {r.target_type === "post" ? "投稿" : "コメント"} · {reasonLabels[r.reason]} · {statusLabels[r.status]} ·{" "}
                {formatDateTime(r.created_at)}
                {r.target_hidden ? " · 非表示済み" : ""}
              </p>
              {r.target_title ? <p className="font-medium">{r.target_title}</p> : null}
              {r.target_body ? (
                <p className="line-clamp-4 whitespace-pre-wrap text-sm text-slate-300">{r.target_body}</p>
              ) : (
                <p className="text-sm text-slate-400">対象は削除されています</p>
              )}
              {r.detail ? <p className="whitespace-pre-wrap text-sm">通報内容: {r.detail}</p> : null}
              <p className="text-xs text-slate-300">通報者: {r.reporter_nickname ?? "名前なし"}</p>
              {r.target_body ? (
                <Link href={`/posts/${r.post_id}`} className="text-sm text-blue-300 underline">
                  投稿を見る
                </Link>
              ) : null}
            </li>
          ))}
        </ul>
      ) : (
        <ul className="space-y-3">
          {lists[tab as "feedback" | "contact" | "requests"].items.map((i) => (
            <li key={i.id} className="space-y-1 rounded-xl border border-white/20 glass bg-white/10 p-3">
              <p className="text-xs text-slate-200">
                {inquiryCategoryLabels[i.category]} · {i.user_nickname ?? "名前なし"} · {formatDateTime(i.created_at)}
                {i.contact_email ? ` · ${i.contact_email}` : ""}
              </p>
              {i.body.startsWith(i.subject) ? null : <p className="font-medium">{i.subject}</p>}
              <p className="whitespace-pre-wrap text-sm text-slate-100">{i.body}</p>
            </li>
          ))}
        </ul>
      )}
      {tab !== "products" && !list.loading && list.items.length === 0 && !list.error ? (
        <p className="text-sm text-slate-200">まだありません</p>
      ) : null}
      {tab !== "products" && list.more ? (
        <button
          type="button"
          onClick={list.loadMore}
          disabled={list.loading}
          className="w-full rounded-xl border border-white/25 bg-white/10 py-2 text-sm text-slate-100 disabled:opacity-50"
        >
          {list.loading ? "読み込み中..." : "もっと見る"}
        </button>
      ) : null}
    </div>
  );
}
