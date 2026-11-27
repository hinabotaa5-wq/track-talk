import { L } from "@/lib/i18n";
import type { PremiumSlot, ReviewRequest } from "./types";

// プレミアム（本体 #1053）。決まった5枠に、申込全体で1〜10本の動画を送り、コーチが総括（全体の一言診断＋枠ごとのブロック）で返す。
// 枠・本数・入力の上限は API（apps/api/internal/httpx/premium.go）と揃える。

export const premiumSlots: PremiumSlot[] = ["race", "start", "second_half", "practice", "other"];

export const premiumSlotLabels: Record<PremiumSlot, string> = {
  get race() { return L("レース全体"); },
  get start() { return L("スタート"); },
  get second_half() { return L("後半"); },
  get practice() { return L("練習"); },
  get other() { return L("その他"); },
};

export const maxPremiumVideos = 10;
// 総括の枠ごとのブロックの上限。全体の一言診断はスタンダードの回答と同じ300字。
export const maxPremiumSectionChars = 1000;

// 動画ごとの入力の上限（すべて任意）。レース全体：この時のタイム・風速・その時の体のコンディション。練習：その練習をした目的。
export const premiumVideoInputLimits = { race_time: 100, wind_speed: 50, body_condition: 300, practice_purpose: 300 } as const;

export function isPremium(request: Pick<ReviewRequest, "plan">): boolean {
  return request.plan === "premium";
}

// プレミアムの申込画面への道。コーチ一覧のプレミアムタブ・特集ページから使う。
export function premiumRequestHref(coachID: string): string {
  return `/coaching/new?plan=premium&coach=${encodeURIComponent(coachID)}&pay=card`;
}
