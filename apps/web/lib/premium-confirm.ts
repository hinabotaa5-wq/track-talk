import { L } from "@/lib/i18n";
import { premiumSlotLabels, premiumSlots } from "@/lib/premium";
import { reviewConfirmRows, type ConfirmRow } from "@/lib/purchase-confirm";
import type { PremiumSlot } from "@/lib/types";

// プレミアム（本体 #1053）の最終確認の項目。内容（担当・種目・選手情報・枠ごとの本数）はプレミアム用に出し、
// お支払い（総額・クーポンの内訳・支払い方法・キャンセルと返金）はスタンダードと同じ行をそのまま使う（カード払い）。
export type PremiumConfirmInput = {
  priceYen: number;
  couponYen?: number;
  eventLabel: string;
  coachName?: string;
  pb: string;
  goalTime: string;
  videos: { slot: PremiumSlot; fileName: string; fileSizeBytes: number }[];
};

export function premiumConfirmRows({ priceYen, couponYen = 0, eventLabel, coachName, pb, goalTime, videos }: PremiumConfirmInput): ConfirmRow[] {
  const content = coachName
    ? L("プレミアム（動画{n}本・担当 {coachName}）", { n: videos.length, coachName })
    : L("プレミアム（動画{n}本）", { n: videos.length });
  const perSlot = premiumSlots
    .map((slot) => ({ slot, count: videos.filter((v) => v.slot === slot).length }))
    .filter((item) => item.count > 0)
    .map((item) => L("{slot} {n}本", { slot: premiumSlotLabels[item.slot], n: item.count }))
    .join(L("、"));
  const rows: ConfirmRow[] = [
    { label: L("内容"), value: content },
    { label: L("種目"), value: eventLabel },
    ...(pb.trim() ? [{ label: L("PB（自己ベスト）"), value: pb.trim() }] : []),
    ...(goalTime.trim() ? [{ label: L("目標タイム"), value: goalTime.trim() }] : []),
    { label: L("枠ごとの動画"), value: perSlot },
    { label: L("回答期限"), value: L("申し込みが確定してから24時間以内に総括が届きます") },
  ];
  // お支払いの行は、スタンダードの確認と同じもの（回答期限より後ろ）を使う。
  const standard = reviewConfirmRows({ method: "card", priceYen, couponYen, hasVideo: true, question: "", eventLabel, videoFileName: null, videoFileSizeBytes: null });
  const deadline = standard.findIndex((row) => row.label === L("回答期限"));
  return [...rows, ...standard.slice(deadline + 1)];
}
