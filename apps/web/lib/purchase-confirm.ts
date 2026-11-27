import { L } from "@/lib/i18n";
import { formatDateTime, formatDisplayPrice } from "@/lib/format";
import { reviewPayMethodLabels } from "@/lib/review";
import type { TicketKind, TicketNextExpiry } from "@/lib/coaching-tickets";

// 支払い画面（Stripe）へ進む前の最終確認に出す項目（本体 #569）。
// 内容・回答期限・お支払い（総額・支払い方法）・キャンセルと戻り／返金の条件を、購入のたびに同じ順で見せる。
export type ConfirmRow = { label: string; value: string; multiline?: boolean };

// 動画サイズの整形。upload-progress.tsx の formatSize と同じ規則（1MB以上 X.X MB／1KB以上 N KB／未満 N B）。
function formatVideoSize(bytes: number): string {
  if (bytes >= 1024 * 1024) return `${(bytes / (1024 * 1024)).toFixed(1)} MB`;
  if (bytes >= 1024) return `${Math.round(bytes / 1024)} KB`;
  return `${Math.round(bytes)} B`;
}

// 申込で選手が任意で書ける選手情報（本体 #1013 補足）。並びは申込フォームと確認画面の順。
export const reviewAthleteInfoKeys = ["pb", "goal_time", "wind_speed", "body_condition"] as const;
export type ReviewAthleteInfoKey = (typeof reviewAthleteInfoKeys)[number];
export type ReviewAthleteInfo = Partial<Record<ReviewAthleteInfoKey, string>>;

export function reviewAthleteInfoLabel(key: ReviewAthleteInfoKey): string {
  switch (key) {
    case "pb": return L("PB（自己ベスト）");
    case "goal_time": return L("目標タイム");
    case "wind_speed": return L("風速");
    case "body_condition": return L("その時の体のコンディション");
  }
}

export type ReviewConfirmInput = {
  method: "card" | "paypay" | "tickets";
  priceYen: number;
  // 本体 #1031：適用する ¥500 クーポンの割引額（運営負担）。使わないときは 0 か未指定。
  // 指定があれば、内訳（価格・クーポン・支払額）を出す。支払額が0円なら支払いの行は出さない。
  couponYen?: number;
  hasVideo: boolean;
  // 申込内容そのもの（誤送信防止のため確認画面に出す）。
  question: string;
  eventLabel: string;
  videoFileName: string | null;
  videoFileSizeBytes: number | null;
  coachName?: string;
  // チケットで払うとき、確保される1枚の有効期限（いちばん早く失効する分から使う）。旧商品の分は期限なし。
  // 種別を選んだとき（本体 #687）は、その種別の中でいちばん早く失効する分の期限を渡す。
  nextExpiry?: Pick<TicketNextExpiry, "expires_at"> | null;
  // チケットの種別（本体 #687）。reward（無料チケット）は支払いの行と有効期限の行を分けて出す。
  ticketKind?: TicketKind | null;
  // 選手情報（任意）。入力がある項目だけ、相談内容の下に行を足す。
  athleteInfo?: ReviewAthleteInfo;
};

export function reviewConfirmRows({ method, priceYen, couponYen = 0, hasVideo, question, eventLabel, videoFileName, videoFileSizeBytes, coachName, nextExpiry, ticketKind, athleteInfo }: ReviewConfirmInput): ConfirmRow[] {
  const content = hasVideo
    ? (coachName ? L("スタンダード（動画1本・担当 {coachName}）", { coachName }) : L("スタンダード（動画1本）"))
    : (coachName ? L("スタンダード（悩みのみ・担当 {coachName}）", { coachName }) : L("スタンダード（悩みのみ）"));
  const rows: ConfirmRow[] = [
    { label: L("内容"), value: content },
    { label: L("種目"), value: eventLabel },
    { label: L("相談内容"), value: question.trim(), multiline: true },
    ...reviewAthleteInfoKeys.flatMap((key) => {
      const value = athleteInfo?.[key]?.trim() ?? "";
      return value ? [{ label: reviewAthleteInfoLabel(key), value, multiline: key === "body_condition" }] : [];
    }),
    {
      label: L("動画"),
      value: videoFileName !== null
        ? L("{fileName}（{size}）", { fileName: videoFileName, size: formatVideoSize(videoFileSizeBytes ?? 0) })
        : L("なし"),
    },
    { label: L("回答期限"), value: L("申し込みが確定してから24時間以内に回答が来ます（再提出時は再提出から24時間以内）") },
  ];
  if (method === "tickets") {
    const expiry = nextExpiry ? formatDateTime(nextExpiry.expires_at) : L("期限なし（以前に購入した分）");
    if (ticketKind === "reward") {
      rows.push(
        { label: L("お支払い"), value: L("無料チケット1枚（このあとのお支払いはありません）") },
        { label: L("使うチケットの有効期限"), value: expiry },
      );
    } else {
      rows.push({ label: L("お支払い"), value: L("チケット1枚（有効期限 {expiry}・追加の支払いなし）", { expiry }) });
    }
    // 本体 #797：無料チケットは、運営者・コーチ都合の終了なら期限切れでも出し直すため、例外を書き分ける。
    const cancelValue = ticketKind === "reward"
      ? L("送信時に1枚を確保し、返却時に使います。着手前はキャンセルでき、その場合は1枚戻ります。着手後はキャンセルできません。期限までに返らない場合やコーチ都合の返却でも1枚戻ります。無料チケットを使って申し込んだ依頼が運営者またはコーチの都合で終了した場合は、有効期限を過ぎていても1枚を出し直します。")
      : L("送信時に1枚を確保し、返却時に使います。着手前はキャンセルでき、その場合は1枚戻ります。着手後はキャンセルできません。期限までに返らない場合やコーチ都合の返却でも1枚戻りますが、有効期限を過ぎた分は失効します。");
    rows.push({ label: L("キャンセル・チケットの戻り"), value: cancelValue });
    return rows;
  }
  if (couponYen > 0) {
    const payYen = Math.max(priceYen - couponYen, 0);
    rows.push(
      { label: L("価格"), value: L("{v0}（税込）", { v0: formatDisplayPrice(priceYen) }) },
      { label: L("クーポン"), value: L("−{v0}（¥500クーポン1枚）", { v0: formatDisplayPrice(couponYen) }) },
      { label: L("お支払い額"), value: L("{v0}（税込）", { v0: formatDisplayPrice(payYen) }) },
    );
    const couponCancel = L("送信時にクーポン1枚を確保し、回答の返却時に使います。着手前のキャンセル、期限までに返らない場合、コーチが返却した場合はクーポンが戻ります（有効期限を過ぎた分は失効します。運営者またはコーチの都合で終了した場合は出し直します）。");
    if (payYen === 0) {
      rows.push({ label: L("キャンセル・クーポンの戻り"), value: couponCancel });
      return rows;
    }
    rows.push({ label: L("クーポンの戻り"), value: couponCancel });
  } else {
    rows.push({ label: L("総額"), value: L("{v0}（税込）", { v0: formatDisplayPrice(priceYen) }) });
  }
  rows.push({ label: L("お支払い方法"), value: reviewPayMethodLabels[method] });
  if (method === "card") {
    rows.push(
      { label: L("キャンセル・返金"), value: L("次の画面でカードの代金を仮押さえし、回答が返った時に確定します。着手前はキャンセルでき、その場合は代金はかかりません。着手後はキャンセルできません。期限までに返らない場合やコーチが返却した場合は、代金はかかりません（確定済みの場合は返金します）。回答返却後の返金は利用規約に定める場合に限ります。") },
    );
  } else {
    rows.push(
      { label: L("確定のタイミング"), value: L("次の画面でお支払いが済んだ時に確定します。") },
      { label: L("取消し・返金"), value: L("着手前に依頼画面でキャンセルした場合、期限までに添削が返らない場合、またはコーチが例外返却した場合は、全額を返金します。着手後の利用者都合によるキャンセルはできません。回答返却後の返金は利用規約に定める場合に限ります。") },
    );
  }
  return rows;
}
