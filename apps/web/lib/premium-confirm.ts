import { L } from "@/lib/i18n";
import { reviewConfirmRows, type ConfirmRow } from "@/lib/purchase-confirm";

// プレミアム（本体 #1053）の最終確認の項目。内容（担当・種目・選手情報・グループごとの本数）はプレミアム用に出し、
// お支払い（総額・クーポンの内訳・支払い方法・キャンセルと返金）はスタンダードと同じ行をそのまま使う（カード払い）。
export type PremiumConfirmInput = {
  priceYen: number;
  couponYen?: number;
  eventLabel: string;
  coachName?: string;
  pb: string;
  goalTime: string;
  // グループ（テーマ名と本数）。並びは選手が作った順。
  groups: { title: string; count: number }[];
};

export function premiumConfirmRows({ priceYen, couponYen = 0, eventLabel, coachName, pb, goalTime, groups }: PremiumConfirmInput): ConfirmRow[] {
  const total = groups.reduce((sum, group) => sum + group.count, 0);
  const content = coachName
    ? L("プレミアム（動画{n}本・担当 {coachName}）", { n: total, coachName })
    : L("プレミアム（動画{n}本）", { n: total });
  const perGroup = groups
    .filter((group) => group.count > 0)
    .map((group) => L("{slot} {n}本", { slot: group.title, n: group.count }))
    .join(L("、"));
  const rows: ConfirmRow[] = [
    { label: L("内容"), value: content },
    { label: L("種目"), value: eventLabel },
    ...(pb.trim() ? [{ label: L("PB（自己ベスト）"), value: pb.trim() }] : []),
    ...(goalTime.trim() ? [{ label: L("目標タイム"), value: goalTime.trim() }] : []),
    { label: L("グループごとの動画"), value: perGroup },
    { label: L("回答期限"), value: L("申し込みが確定してから24時間以内に回答とまとめが届きます") },
  ];
  // お支払いの行は、スタンダードの確認と同じもの（回答期限より後ろ）を使う。
  const standard = reviewConfirmRows({ method: "card", priceYen, couponYen, hasVideo: true, question: "", eventLabel, videoFileName: null, videoFileSizeBytes: null });
  const deadline = standard.findIndex((row) => row.label === L("回答期限"));
  return [...rows, ...standard.slice(deadline + 1)];
}
