"use client";

import Link from "next/link";
import { useEffect, useState } from "react";
import { useAuth } from "@/components/auth-provider";
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
import { ViewSwitch } from "@/components/view-switch";

const kinds: ProductKind[] = ["shoe", "spike"];

type Ranking = { event: string; event_label: string; min_reviews: number; products: Product[] };

// 種目ごとのランキング。その種目の人のレビューだけで平均★を出す。種目を絞り込んでいないときは、登録した種目で出す。
function EventRanking({ kind, event, filters }: { kind: ProductKind; event: string; filters: ReviewFilters }) {
  const [ranking, setRanking] = useState<Ranking | null>(null);

  useEffect(() => {
    let active = true;
    apiFetch<Ranking>(`/v1/products/ranking?${new URLSearchParams({ kind, event })}`)
      .then((d) => {
        if (active) setRanking(d);
      })
      .catch(() => {
        // ランキングは一覧のおまけなので、読めなければ出さない。
        if (active) setRanking(null);
      });
    return () => {
      active = false;
    };
  }, [kind, event]);

  if (!ranking || ranking.event !== event || ranking.products.length === 0) return null;
  return (
    <section className="mb-4 rounded-2xl border border-white/20 glass bg-white/10 p-3">
      <h2 className="text-sm font-semibold text-slate-100">
        {ranking.event_label}の人の{productKindLabels[kind]}ランキング
      </h2>
      <p className="mb-2 text-xs text-slate-300">{ranking.event_label}を選んだ人のレビューだけで平均★を出しています（{ranking.min_reviews}件以上の商品）。</p>
      <ol className="space-y-1.5">
        {ranking.products.map((p, i) => (
          <li key={p.id}>
            <Link
              href={withReviewFilters(`/reviews/${p.id}`, { ...filters, event })}
              className="flex items-center gap-3 rounded-xl px-2 py-1.5 text-sm transition hover:bg-white/10"
            >
              <span className="w-5 shrink-0 text-center font-bold text-amber-300">{i + 1}</span>
              <span className="min-w-0 flex-1 truncate text-slate-100">
                <span className="text-xs text-slate-300">{p.brand}</span> {p.name}
              </span>
              <span className="shrink-0 text-xs text-amber-300">{ratingText(p.avg_rating, p.review_count)}</span>
            </Link>
          </li>
        ))}
      </ol>
    </section>
  );
}

export function ReviewHome({ kind, q, page, filters }: { kind: ProductKind; q: string; page: number; filters: ReviewFilters }) {
  const [products, setProducts] = useState<Product[] | null>(null);
  const [meta, setMeta] = useState<Meta>({ page, per_page: 20, total: 0 });
  const [error, setError] = useState("");
  const { profile } = useAuth();
  const rankingEvent = filters.event ?? profile?.event ?? null;
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
      <div className="mb-3">
        <ViewSwitch
          label="種類"
          layoutId="review-kind-pill"
          views={kinds.map((k) => ({ id: k, label: productKindLabels[k] }))}
          current={kind}
          href={(k) => href(undefined, k)}
          onSelect={(k) => {
            if (k !== kind) window.history.pushState(null, "", href(undefined, k));
          }}
        />
      </div>
      <div className="mb-3">
        <ReviewFilterBar filters={filters} onChange={(next) => window.history.pushState(null, "", withReviewFilters(homeHref("review", q, undefined, kind), next))} />
      </div>
      {rankingEvent && !q && page === 1 ? <EventRanking kind={kind} event={rankingEvent} filters={filters} /> : null}
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
