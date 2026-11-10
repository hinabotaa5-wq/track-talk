import { L } from "@/lib/i18n";
import type { ProductKind } from "./home-sort";

export const productKindLabels: Record<ProductKind, string> = {
  get shoe() { return L("シューズ"); },
  get spike() { return L("スパイク"); },
};

export function ratingText(avg: number, count: number) {
  return count === 0 ? L("レビューなし") : L("★{v0}（{count}件）", { v0: avg.toFixed(1), count });
}

// 種目ごとのランキングは、いま開いている種類と種目のものだけを出す。切り替えた直後に、
// 読み終わるまで前の種類のランキングが新しい見出しで出ないようにする。
export function currentRanking<T extends { kind: ProductKind; event: string }>(ranking: T | null, kind: ProductKind, event: string): T | null {
  return ranking && ranking.kind === kind && ranking.event === event ? ranking : null;
}
