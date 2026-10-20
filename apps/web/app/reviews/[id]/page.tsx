"use client";

import Link from "next/link";
import { useSearchParams } from "next/navigation";
import { use, useCallback, useEffect, useRef, useState } from "react";
import { useAuth } from "@/components/auth-provider";
import { EmptyState, ErrorState } from "@/components/empty-state";
import { FeedLoadingFrame } from "@/components/loading-frame";
import { ProductImage } from "@/components/product-image";
import { ReviewAttrChips, ReviewAttrsFields, ReviewFilterBar } from "@/components/review-filters";
import { Button } from "@/components/ui/button";
import { Textarea } from "@/components/ui/textarea";
import { ApiRequestError, apiFetch } from "@/lib/api";
import { formatDateTime } from "@/lib/format";
import { fetchAllPages } from "@/lib/paging";
import { productKindLabels, ratingText } from "@/lib/product";
import {
  activeReviewFilterCount,
  emptyReviewAttrs,
  loadReviewAttrs,
  parseReviewFilters,
  reviewAttrsPayload,
  reviewFiltersApiQuery,
  saveReviewAttrs,
  withReviewFilters,
} from "@/lib/review-filters";
import type { Meta, Product, ProductReview } from "@/lib/types";

function Stars({ value }: { value: number }) {
  return (
    <span className="text-amber-300" aria-label={`5点中${value}点`}>
      {"★".repeat(value)}
      <span className="text-slate-500">{"★".repeat(5 - value)}</span>
    </span>
  );
}

export default function ProductReviewsPage({ params }: { params: Promise<{ id: string }> }) {
  const { id } = use(params);
  const { session, token } = useAuth();
  const [product, setProduct] = useState<Product | null>(null);
  const [reviews, setReviews] = useState<ProductReview[]>([]);
  const [summary, setSummary] = useState<{ review_count: number; avg_rating: number } | null>(null);
  const sp = useSearchParams();
  const filters = parseReviewFilters(sp);
  const filterQuery = reviewFiltersApiQuery(filters).toString();
  const filtered = activeReviewFilterCount(filters) > 0;
  const [attrs, setAttrs] = useState(emptyReviewAttrs);
  const [error, setError] = useState("");
  const [rating, setRating] = useState(0);
  const [body, setBody] = useState("");
  const [formError, setFormError] = useState("");
  const [pending, setPending] = useState(false);

  // 絞り込みやログイン状態の確定で読み込みが重なったとき、古い応答が後から返って上書きしないよう、最新の読み込みだけ反映する。
  const loadSeq = useRef(0);
  const load = useCallback(async () => {
    const seq = ++loadSeq.current;
    try {
      let firstSummary: { review_count: number; avg_rating: number } | null = null;
      // 自分のレビューが51件目以降にあっても投稿済みと分かるよう、最後のページまで集める。
      const [p, all] = await Promise.all([
        apiFetch<Product>(`/v1/products/${id}`),
        fetchAllPages(async (page) => {
          const r = await apiFetch<{ reviews: ProductReview[]; meta: Meta; summary: { review_count: number; avg_rating: number } }>(
            `/v1/products/${id}/reviews?page=${page}&per_page=50${filterQuery ? `&${filterQuery}` : ""}`,
            { token },
          );
          if (page === 1) firstSummary = r.summary ?? null;
          return { items: r.reviews ?? [], total: r.meta.total };
        }),
      ]);
      if (seq !== loadSeq.current) return;
      setProduct(p);
      setReviews(all.items);
      setSummary(firstSummary);
      setError("");
    } catch (e) {
      if (seq !== loadSeq.current) return;
      setError(e instanceof ApiRequestError && e.status === 404 ? "商品が見つかりませんでした。" : "取得に失敗しました。");
    }
  }, [id, token, filterQuery]);

  useEffect(() => () => document.body.removeAttribute("data-comment-typing"), []);

  useEffect(() => {
    // 前回レビューを書いたときの属性を初期値にする(端末に覚えたもの。サーバーでは読めないので表示後に読む)。
    // eslint-disable-next-line react-hooks/set-state-in-effect
    setAttrs(loadReviewAttrs());
  }, []);

  useEffect(() => {
    // eslint-disable-next-line react-hooks/set-state-in-effect
    void load();
  }, [load]);

  async function submit(e: React.FormEvent) {
    e.preventDefault();
    if (pending) return;
    if (rating < 1) return setFormError("評価を選んでください");
    if (body.trim() === "") return setFormError("コメントを入力してください");
    setPending(true);
    setFormError("");
    try {
      await apiFetch(`/v1/products/${id}/reviews`, {
        method: "POST",
        token,
        body: JSON.stringify({ rating, body, ...reviewAttrsPayload(attrs) }),
      });
      saveReviewAttrs(attrs);
      setRating(0);
      setBody("");
      await load();
    } catch (err) {
      setFormError(err instanceof ApiRequestError ? err.message : "投稿できませんでした");
    } finally {
      setPending(false);
    }
  }

  async function remove(reviewId: string) {
    if (!confirm("このレビューを削除しますか？")) return;
    try {
      await apiFetch(`/v1/product-reviews/${reviewId}`, { method: "DELETE", token });
      await load();
    } catch {
      setFormError("削除できませんでした");
    }
  }

  if (error) return <ErrorState message={error} />;
  if (!product) return <FeedLoadingFrame />;

  const mineExists = reviews.some((r) => r.mine);

  return (
    <article className="mx-auto max-w-2xl space-y-4">
      <div className="overflow-hidden rounded-2xl border border-white/20 glass bg-white/10">
        {product.product_url ? (
          <a href={product.product_url} target="_blank" rel="noopener noreferrer" aria-label={`${product.name}の公式ページを開く`}>
            <ProductImage product={product} className="aspect-[16/9]" />
          </a>
        ) : (
          <ProductImage product={product} className="aspect-[16/9]" />
        )}
        <div className="space-y-1 p-4">
          <span className="rounded-full border border-white/20 bg-white/12 px-2.5 py-0.5 text-xs font-semibold text-slate-200">
            {productKindLabels[product.kind]}
          </span>
          {product.discontinued ? (
            <span className="ml-2 rounded-full border border-amber-300/40 bg-amber-300/10 px-2.5 py-0.5 text-xs font-semibold text-amber-200">
              旧モデル
            </span>
          ) : null}
          <p className="text-xs text-slate-300">{product.brand}</p>
          <h1 className="text-xl font-bold">{product.name}</h1>
          <p className="text-sm text-amber-300">{ratingText(product.avg_rating, product.review_count)}</p>
          {product.product_url ? (
            <a
              href={product.product_url}
              target="_blank"
              rel="noopener noreferrer"
              className="mt-2 inline-flex min-h-9 items-center rounded-full border border-white/25 bg-white/10 px-4 text-sm font-medium text-blue-300 transition active:scale-95"
            >
              公式ページを見る ↗
            </a>
          ) : null}
        </div>
      </div>

      <section className="space-y-3">
        <h2 className="font-semibold">レビュー {product.review_count}件</h2>
        <ReviewFilterBar filters={filters} onChange={(next) => window.history.pushState(null, "", withReviewFilters(`/reviews/${id}`, next))} />
        {filtered && summary ? (
          <p className="text-sm text-slate-200">
            条件に合うレビュー <span className="text-amber-300">{ratingText(summary.avg_rating, summary.review_count)}</span>
          </p>
        ) : null}
        {reviews.length === 0 ? (
          filtered ? (
            <EmptyState title="条件に合うレビューがありません" body="絞り込みを変えるか、解除してください。" />
          ) : (
            <EmptyState title="まだレビューがありません" body="最初のレビューを書いてみましょう。" />
          )
        ) : null}
        {reviews.map((r) => (
          <div key={r.id} className="space-y-1 rounded-xl border border-white/20 glass bg-white/10 p-3.5">
            <Stars value={r.rating} />
            <ReviewAttrChips review={r} />
            <p className="whitespace-pre-wrap text-sm text-slate-100">{r.body}</p>
            <div className="flex items-center justify-between text-xs text-slate-300">
              <span>{formatDateTime(r.created_at)}</span>
              {r.mine ? (
                <button type="button" onClick={() => remove(r.id)} className="cursor-pointer text-red-300 underline">
                  削除
                </button>
              ) : null}
            </div>
          </div>
        ))}
      </section>

      {!session ? (
        <p className="rounded-xl border border-white/20 glass bg-white/10 p-3.5 text-sm text-slate-200">
          レビューを書くには<Link href="/login" className="text-blue-300 underline">ログイン</Link>してください。
        </p>
      ) : filtered ? (
        <p className="rounded-xl border border-white/20 glass bg-white/10 p-3.5 text-sm text-slate-200">
          レビューを書くときは、絞り込みを解除してください。
        </p>
      ) : mineExists ? (
        <p className="rounded-xl border border-white/20 glass bg-white/10 p-3.5 text-sm text-slate-200">
          この商品にはレビューを書いています。書き直す場合は削除してください。
        </p>
      ) : (
        <form
          onSubmit={submit}
          onFocus={() => document.body.setAttribute("data-comment-typing", "")}
          onBlur={(event) => {
            if (!event.currentTarget.contains(event.relatedTarget as Node | null)) {
              document.body.removeAttribute("data-comment-typing");
            }
          }}
          className="space-y-3 rounded-xl border border-white/20 glass bg-white/10 p-3.5"
        >
          <p className="text-sm font-medium">レビューを書く</p>
          <div className="flex gap-1" role="radiogroup" aria-label="評価">
            {[1, 2, 3, 4, 5].map((n) => (
              <button
                key={n}
                type="button"
                role="radio"
                aria-checked={rating === n}
                aria-label={`${n}点`}
                onClick={() => setRating(n)}
                className={`size-10 cursor-pointer text-2xl ${n <= rating ? "text-amber-300" : "text-slate-500"}`}
              >
                ★
              </button>
            ))}
          </div>
          <Textarea
            rows={4}
            maxLength={2000}
            value={body}
            onChange={(e) => setBody(e.target.value)}
            placeholder="履き心地、反発、フィット感など"
            className="glass bg-white/8 border-white/20 text-white placeholder:text-slate-300 focus:border-blue-500"
          />
          <ReviewAttrsFields value={attrs} onChange={setAttrs} />
          {formError ? <p className="text-sm text-red-400">{formError}</p> : null}
          <Button type="submit" className="w-full" disabled={pending}>
            {pending ? "送っています" : "投稿する"}
          </Button>
        </form>
      )}
    </article>
  );
}
