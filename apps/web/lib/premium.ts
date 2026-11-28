import { L } from "@/lib/i18n";
import type { PremiumKind, ReviewRequest } from "./types";

// プレミアム（本体 #1053、補足 7231984993・補足2 7231991592 の再設計）。動画をテーマごとのグループに分けて申込全体で1〜6本送り、
// コーチがグループごとの自由記入＋最後の「まとめ」で返す。制限は合計6本だけ（グループ数・グループごとの本数に制限なし）。
// テンプレ・本数・入力の上限は API（apps/api/internal/httpx/premium.go）と揃える。

// グループのテーマのテンプレ。全部を埋める必要はなく、同じテンプレのグループを何個作ってもよい。
export const premiumKinds: PremiumKind[] = ["race", "start", "second_half", "practice", "other"];

export const premiumKindLabels: Record<PremiumKind, string> = {
  get race() { return L("レース全体"); },
  get start() { return L("スタート"); },
  get second_half() { return L("後半"); },
  get practice() { return L("練習"); },
  get other() { return L("その他"); },
};

export const maxPremiumVideos = 6;
// 返却のグループごとの記入と、最後のまとめの上限。
export const maxPremiumSectionChars = 2000;
export const maxPremiumSummaryChars = 2000;

// グループの入力の上限（テーマ名以外は任意）。意識・目的は全グループ共通、タイム・風速・コンディションはレース全体のグループの動画だけ。
export const premiumGroupInputLimits = { title: 30, focus_note: 300 } as const;
export const premiumVideoInputLimits = { race_time: 100, wind_speed: 50, body_condition: 300 } as const;

// defaultGroupTitle は、テンプレを選んだときのテーマ名。同じ名前がもうあれば番号を付ける（返却の引用単位はテーマ名なので重ねない）。
export function defaultGroupTitle(kind: PremiumKind, taken: string[]): string {
  const base = premiumKindLabels[kind];
  if (!taken.includes(base)) return base;
  for (let n = 2; ; n++) {
    const title = `${base}${n}`;
    if (!taken.includes(title)) return title;
  }
}

export function isPremium(request: Pick<ReviewRequest, "plan">): boolean {
  return request.plan === "premium";
}

// プレミアムの申込画面への道。コーチ一覧のプレミアムタブ・特集ページから使う。
export function premiumRequestHref(coachID: string): string {
  return `/coaching/new?plan=premium&coach=${encodeURIComponent(coachID)}&pay=card`;
}
