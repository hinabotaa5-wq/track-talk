"use client";

import { useState } from "react";
import { SlidersHorizontal } from "lucide-react";
import {
  activeReviewFilterCount,
  reviewEvents,
  reviewFootSizes,
  reviewFootWidths,
  reviewLevels,
  reviewUsages,
  type ReviewAttrsInput,
  type ReviewFilters,
} from "@/lib/review-filters";
import type { ProductReview } from "@/lib/types";

const selectClass =
  "w-full rounded-md border border-white/25 glass bg-white/10 px-2.5 py-2 text-sm text-slate-100 outline-none focus:border-blue-400";

function Select({
  label,
  value,
  onChange,
  options,
  empty,
}: {
  label: string;
  value: string;
  onChange: (value: string) => void;
  options: readonly { value: string; label: string }[];
  empty: string;
}) {
  return (
    <label className="block min-w-0">
      <span className="mb-1 block text-xs text-slate-300">{label}</span>
      <select className={selectClass} value={value} onChange={(e) => onChange(e.target.value)}>
        <option value="">{empty}</option>
        {options.map((item) => (
          <option key={item.value} value={item.value}>
            {item.label}
          </option>
        ))}
      </select>
    </label>
  );
}

const footSizeOptions = reviewFootSizes.map((size) => ({ value: String(size), label: `${size.toFixed(1)}cm` }));

// レビューを書いた人の属性で絞り込む欄。開いたり閉じたりできる。
export function ReviewFilterBar({ filters, onChange }: { filters: ReviewFilters; onChange: (next: ReviewFilters) => void }) {
  const count = activeReviewFilterCount(filters);
  const [open, setOpen] = useState(count > 0);
  const set = (key: keyof ReviewFilters, value: string) =>
    onChange({ ...filters, [key]: value === "" ? undefined : value });

  return (
    <div className="space-y-2">
      <div className="flex items-center gap-2">
        <button
          type="button"
          onClick={() => setOpen((v) => !v)}
          aria-expanded={open}
          className={`inline-flex min-h-9 items-center gap-1.5 rounded-full border px-3.5 text-sm font-medium ${
            count > 0 ? "border-blue-400 bg-blue-500/30 text-white" : "border-white/25 bg-white/10 text-slate-200"
          }`}
        >
          <SlidersHorizontal className="size-4" aria-hidden />
          絞り込み{count > 0 ? `（${count}）` : ""}
        </button>
        {count > 0 ? (
          <button type="button" onClick={() => onChange({})} className="text-sm text-blue-300 underline">
            解除
          </button>
        ) : null}
      </div>
      {open ? (
        <div className="space-y-2 rounded-xl border border-white/20 glass bg-white/10 p-3">
          <div className="grid grid-cols-2 gap-2">
            <Select label="種目" value={filters.event ?? ""} onChange={(v) => set("event", v)} options={reviewEvents} empty="すべて" />
            <Select label="足幅" value={filters.width ?? ""} onChange={(v) => set("width", v)} options={reviewFootWidths} empty="すべて" />
          </div>
          <p className="text-xs text-slate-300">書いた人がこれらを選んだレビューだけで数えます。選んでいない以前のレビューは、絞り込むと対象から外れます。</p>
        </div>
      ) : null}
    </div>
  );
}

// レビューを書くときの属性(どれも任意)。
export function ReviewAttrsFields({ value, onChange }: { value: ReviewAttrsInput; onChange: (next: ReviewAttrsInput) => void }) {
  const set = (key: keyof ReviewAttrsInput) => (v: string) => onChange({ ...value, [key]: v });
  return (
    <div className="space-y-1">
      <p className="text-xs text-slate-300">あなたについて(任意・絞り込みに使います。足のサイズは1cm幅でしか表示しません)</p>
      <div className="grid grid-cols-2 gap-2">
        <Select label="種目" value={value.event} onChange={set("event")} options={reviewEvents} empty="選ばない" />
        <Select label="レベル" value={value.level} onChange={set("level")} options={reviewLevels} empty="選ばない" />
        <Select label="足のサイズ" value={value.footSize} onChange={set("footSize")} options={footSizeOptions} empty="選ばない" />
        <Select label="足幅" value={value.width} onChange={set("width")} options={reviewFootWidths} empty="選ばない" />
        <Select label="使い方" value={value.usage} onChange={set("usage")} options={reviewUsages} empty="選ばない" />
      </div>
    </div>
  );
}

// レビューに付いた属性の小さな札。
export function ReviewAttrChips({ review }: { review: ProductReview }) {
  const chips = [review.event_label, review.level_label, review.foot_size_label, review.foot_width_label ? `足幅${review.foot_width_label}` : null, review.usage_label].filter(
    (chip): chip is string => Boolean(chip),
  );
  if (chips.length === 0) return null;
  return (
    <p className="flex flex-wrap gap-1">
      {chips.map((chip) => (
        <span key={chip} className="rounded-full border border-white/15 bg-white/10 px-2 py-0.5 text-[11px] text-slate-200">
          {chip}
        </span>
      ))}
    </p>
  );
}
