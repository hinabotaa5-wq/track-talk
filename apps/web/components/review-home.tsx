"use client";

import Link from "next/link";
import { useEffect, useState } from "react";
import { apiFetch } from "@/lib/api";
import { homeHref, type ProductKind } from "@/lib/home-sort";
import { productKindLabels, ratingText } from "@/lib/product";
import { activeReviewFilterCount, reviewFiltersApiQuery, withReviewFilters, type ReviewFilters } from "@/lib/review-filters";
import type { Meta, Product } from "@/lib/types";
import { EmptyState, ErrorState } from "@/components/empty-state";
import { FeedLoadingFrame } from "@/components/loading-frame";
import { ProductImage } from "@/components/product-image";
import { ProductRequest } from "@/components/product-request";
import { ReviewFilterBar } from "@/components/review-filters";

const kinds: ProductKind[] = ["shoe", "spike"];

export function ReviewHome({ kind, q, page, filters }: { kind: ProductKind; q: string; page: number; filters: ReviewFilters }) {
  const [products, setProducts] = useState<Product[] | null>(null);
  const [meta, setMeta] = useState<Meta>({ page, per_page: 20, total: 0 });
  const [error, setError] = useState("");
  const filterQuery = reviewFiltersApiQuery(filters).toString();

  useEffect(() => {
    let active = true;
    const qs = new URLSearchParams({ kind, page: String(page), per_page: "20" });
    if (q) qs.set("q", q);
    new URLSearchParams(filterQuery).forEach((value, key) => qs.set(key, value));
    apiFetch<{ products: Product[]; meta: Meta }>(`/v1/products?${qs}`)
      .then((d) => {
        if (!active) return;
        setProducts(d.products ?? []);
        setMeta(d.meta);
      })
      .catch(() => {
        if (active) setError("商品の取得に失敗しました。");
      });
    return () => {
      active = false;
    };
  }, [kind, q, page, filterQuery]);

  const hasNext = meta.page * meta.per_page < meta.total;
  const filtered = activeReviewFilterCount(filters) > 0;
  const href = (p?: number, k: ProductKind = kind) => withReviewFilters(homeHref("review", q, p, k), filters);

  return (
    <>
      <div className="mb-3 flex gap-2" role="tablist" aria-label="種類">
        {kinds.map((k) => (
          <a
            key={k}
            href={href(undefined, k)}
            role="tab"
            aria-selected={kind === k}
            onClick={(e) => {
              if (e.button !== 0 || e.metaKey || e.ctrlKey || e.shiftKey || e.altKey) return;
              e.preventDefault();
              if (k !== kind) window.history.pushState(null, "", href(undefined, k));
            }}
            className={`inline-flex min-h-9 cursor-pointer items-center rounded-full border px-4 text-sm font-medium transition active:scale-95 ${
              kind === k ? "border-blue-400 bg-blue-500/30 text-white" : "border-white/25 bg-white/10 text-slate-200"
            }`}
          >
            {productKindLabels[k]}
          </a>
        ))}
      </div>
      <div className="mb-3">
        <ReviewFilterBar filters={filters} onChange={(next) => window.history.pushState(null, "", withReviewFilters(homeHref("review", q, undefined, kind), next))} />
      </div>
      {error ? <ErrorState message={error} /> : null}
      {!error && products === null ? <FeedLoadingFrame /> : null}
      {products && products.length === 0 ? (
        <EmptyState
          title={filtered ? "条件に合うレビューがありません" : "商品がありません"}
          body={filtered ? "絞り込みを変えるか、解除してください。" : q ? "キーワードを変えて探してください。" : "商品は運営が順次追加します。"}
        />
      ) : null}
      {products && products.length > 0 ? (
        <div className="columns-2 gap-3 md:columns-3 xl:columns-4">
          {products.map((p) => (
            <Link
              key={p.id}
              href={withReviewFilters(`/reviews/${p.id}`, filters)}
              className="group mb-3 block w-full break-inside-avoid overflow-hidden rounded-2xl border border-white/20 glass bg-white/10 shadow-md backdrop-blur-xs transition-all duration-300 ease-out hover:-translate-y-1 hover:border-white/40 hover:shadow-xl hover:shadow-blue-950/30 active:scale-95"
            >
              <ProductImage product={p} />
              <div className="space-y-0.5 px-3.5 py-2.5">
                <p className="text-xs text-slate-300">
                  {p.brand}
                  {p.discontinued ? <span className="ml-1.5 text-amber-300">旧モデル</span> : null}
                </p>
                <p className="line-clamp-2 text-sm font-medium text-slate-100 group-hover:text-blue-400">{p.name}</p>
                <p className="text-xs text-amber-300">
                  {filtered && p.review_count > 0 ? "条件に合う " : ""}
                  {ratingText(p.avg_rating, p.review_count)}
                </p>
              </div>
            </Link>
          ))}
        </div>
      ) : null}
      {products ? <ProductRequest kind={kind} /> : null}
      {products && (meta.page > 1 || hasNext) ? (
        <nav className="mt-4 flex justify-between text-sm">
          {meta.page > 1 ? (
            <a className="text-blue-300 underline" href={href(meta.page - 1)}>前へ</a>
          ) : <span />}
          {hasNext ? (
            <a className="text-blue-300 underline" href={href(meta.page + 1)}>次へ</a>
          ) : null}
        </nav>
      ) : null}
    </>
  );
}
