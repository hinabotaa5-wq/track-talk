// シューズ・スパイクのレビューを、書いた人の種目・レベル・足のサイズ・足幅・使い方で絞り込む。
// 商品の公式ページの仕様は使わない(商品の更新でページが変わるため)。値のコードは API と同じ。

import { practiceEvents } from "./practice-events";

export const reviewLevels = [
  { value: "junior-high", label: "中学" },
  { value: "high-school", label: "高校" },
  { value: "university", label: "大学" },
  { value: "adult", label: "社会人" },
  { value: "masters", label: "マスターズ" },
] as const;

export const reviewFootWidths = [
  { value: "narrow", label: "細め" },
  { value: "regular", label: "ふつう" },
  { value: "wide", label: "広め" },
] as const;

export const reviewUsages = [
  { value: "practice", label: "練習用" },
  { value: "race", label: "試合用" },
  { value: "both", label: "練習・試合" },
] as const;

export const reviewEvents = practiceEvents;

// レビューを書くときの足のサイズ(22.0〜30.0cm、0.5cm刻み)。
export const reviewFootSizes = Array.from({ length: 17 }, (_, i) => 22 + i * 0.5);

// 絞り込みの足のサイズは 1cm 幅(「25cm台」= 25.0〜25.5cm)。
export const reviewSizeBands = Array.from({ length: 8 }, (_, i) => 22 + i);

export type ReviewFilters = {
  event?: string;
  level?: string;
  size?: number;
  width?: string;
  usage?: string;
};

function pick<T extends readonly { value: string }[]>(options: T, raw: string | null) {
  return raw && options.some((item) => item.value === raw) ? raw : undefined;
}

// URL のクエリ(event・level・size・width・usage)から読む。知らない値は無視する。
export function parseReviewFilters(sp: { get(key: string): string | null }): ReviewFilters {
  const size = Number(sp.get("size"));
  return {
    event: pick(reviewEvents, sp.get("event")),
    level: pick(reviewLevels, sp.get("level")),
    size: reviewSizeBands.includes(size) ? size : undefined,
    width: pick(reviewFootWidths, sp.get("width")),
    usage: pick(reviewUsages, sp.get("usage")),
  };
}

export function activeReviewFilterCount(filters: ReviewFilters) {
  return Object.values(filters).filter((value) => value !== undefined).length;
}

// 画面の URL に載せるクエリ。
export function reviewFiltersQuery(filters: ReviewFilters) {
  const params = new URLSearchParams();
  if (filters.event) params.set("event", filters.event);
  if (filters.level) params.set("level", filters.level);
  if (filters.size !== undefined) params.set("size", String(filters.size));
  if (filters.width) params.set("width", filters.width);
  if (filters.usage) params.set("usage", filters.usage);
  return params;
}

// API に渡すクエリ。足のサイズは帯の下限と上限(cm)にする。
export function reviewFiltersApiQuery(filters: ReviewFilters) {
  const params = new URLSearchParams();
  if (filters.event) params.set("event", filters.event);
  if (filters.level) params.set("level", filters.level);
  if (filters.size !== undefined) {
    params.set("foot_size_min", filters.size.toFixed(1));
    params.set("foot_size_max", (filters.size + 0.5).toFixed(1));
  }
  if (filters.width) params.set("foot_width", filters.width);
  if (filters.usage) params.set("usage", filters.usage);
  return params;
}

// 既存のリンクに絞り込みを足す。
export function withReviewFilters(href: string, filters: ReviewFilters) {
  const extra = reviewFiltersQuery(filters).toString();
  if (!extra) return href;
  return `${href}${href.includes("?") ? "&" : "?"}${extra}`;
}

export function reviewSizeBandLabel(band: number) {
  return `${band}cm台`;
}

// レビューを書くときの属性。前回の入力を端末に覚えて、次の初期値にする(保存できなくても動く)。
export type ReviewAttrsInput = { event: string; level: string; footSize: string; width: string; usage: string };

export const emptyReviewAttrs: ReviewAttrsInput = { event: "", level: "", footSize: "", width: "", usage: "" };

const attrsKey = "review-attrs";

export function loadReviewAttrs(): ReviewAttrsInput {
  try {
    const raw = localStorage.getItem(attrsKey);
    if (!raw) return emptyReviewAttrs;
    const saved = JSON.parse(raw) as Partial<ReviewAttrsInput>;
    return {
      event: pick(reviewEvents, saved.event ?? null) ?? "",
      level: pick(reviewLevels, saved.level ?? null) ?? "",
      footSize: reviewFootSizes.includes(Number(saved.footSize)) ? String(saved.footSize) : "",
      width: pick(reviewFootWidths, saved.width ?? null) ?? "",
      usage: pick(reviewUsages, saved.usage ?? null) ?? "",
    };
  } catch {
    return emptyReviewAttrs;
  }
}

export function saveReviewAttrs(attrs: ReviewAttrsInput) {
  try {
    localStorage.setItem(attrsKey, JSON.stringify(attrs));
  } catch {
    // 覚えられなくても、次に選び直すだけ。
  }
}

// API に送る形。選ばなかった項目は送らない。
export function reviewAttrsPayload(attrs: ReviewAttrsInput) {
  return {
    ...(attrs.event ? { event: attrs.event } : {}),
    ...(attrs.level ? { level: attrs.level } : {}),
    ...(attrs.footSize ? { foot_size: Number(attrs.footSize) } : {}),
    ...(attrs.width ? { foot_width: attrs.width } : {}),
    ...(attrs.usage ? { usage: attrs.usage } : {}),
  };
}
