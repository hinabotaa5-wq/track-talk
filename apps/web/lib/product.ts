import type { ProductKind } from "./home-sort";

export const productKindLabels: Record<ProductKind, string> = {
  shoe: "シューズ",
  spike: "スパイク",
};

export function ratingText(avg: number, count: number) {
  return count === 0 ? "レビューなし" : `★${avg.toFixed(1)}（${count}件）`;
}
