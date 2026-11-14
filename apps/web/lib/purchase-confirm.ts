import { L } from "@/lib/i18n";
import { formatDateTime, formatYen } from "@/lib/format";
import { reviewPayMethodLabels } from "@/lib/review";
import type { TicketKind, TicketNextExpiry } from "@/lib/coaching-tickets";

// チケットの払戻しの例外4項目（本体 #572 v3）。販売画面と最終確認画面に同じ順で箇条書きする。
// 「原則払戻し不可」だけを単独で出さず、この4項目を同じ画面に出す。
export function ticketRefundExceptions(): string[] {
  return [
    L("サービス終了時は、終了の時点で未使用のチケット（有効期限にかかわらず）を払い戻します。"),
    L("法令で認められる取消し・解除は、その定めに従います（当社の債務不履行に基づく解除・未成年者の取消しを含みます）。"),
    L("当社・コーチの都合で提供できないときや著しく遅れたときは、チケットを戻すか返金します。"),
    L("当社・コーチの都合で期限内に受けられなかったときは、失効後も1枚500円に相当する額を返金します。"),
  ];
}

// 例外4項目を（1）〜（4）の番号付きでつなげる（最終確認の行用）。
export function numberedRefundExceptions(): string {
  return ticketRefundExceptions().map((e, i) => `（${i + 1}）${e}`).join("");
}

// 支払い画面（Stripe）へ進む前の最終確認に出す項目（本体 #569）。
// 内容・回答期限・お支払い（総額・支払い方法）・キャンセルと戻り／返金の条件を、購入のたびに同じ順で見せる。
export type ConfirmRow = { label: string; value: string };

export type ReviewConfirmInput = {
  method: "card" | "paypay" | "tickets";
  priceYen: number;
  coachName?: string;
  // チケットで払うとき、確保される1枚の有効期限（いちばん早く失効する分から使う）。旧商品の分は期限なし。
  // 種別を選んだとき（本体 #687）は、その種別の中でいちばん早く失効する分の期限を渡す。
  nextExpiry?: Pick<TicketNextExpiry, "expires_at"> | null;
  // チケットの種別（本体 #687）。reward（無料チケット）は支払いの行と有効期限の行を分けて出す。
  ticketKind?: TicketKind | null;
};

export function reviewConfirmRows({ method, priceYen, coachName, nextExpiry, ticketKind }: ReviewConfirmInput): ConfirmRow[] {
  const rows: ConfirmRow[] = [
    { label: L("内容"), value: coachName ? L("フォーム添削（動画1本・担当 {coachName}）", { coachName }) : L("フォーム添削（動画1本）") },
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
    rows.push(
      { label: L("キャンセル・チケットの戻り"), value: L("送信時に1枚を確保し、返却時に使います。着手前はキャンセルでき、その場合は1枚戻ります。着手後はキャンセルできません。期限までに返らない場合やコーチ都合の返却でも1枚戻りますが、有効期限を過ぎた分は失効します。") },
    );
    return rows;
  }
  rows.push(
    { label: L("総額"), value: L("{v0}（税込）", { v0: formatYen(priceYen) }) },
    { label: L("お支払い方法"), value: reviewPayMethodLabels[method] },
  );
  if (method === "card") {
    rows.push(
      { label: L("キャンセル・返金"), value: L("次の画面でカードの代金を確保（仮押さえ）し、返却時に確定します。着手前はキャンセルでき、その場合は確保を取り消すので代金はかかりません。着手後はキャンセルできません。期限までに返らない場合やコーチ都合の返却では全額を返金します。添削返却後の返金は利用規約に定める場合に限ります。") },
    );
  } else {
    rows.push(
      { label: L("確定のタイミング"), value: L("次の画面でお支払いが済んだ時に確定します。") },
      { label: L("取消し・返金"), value: L("着手前に依頼画面でキャンセルした場合、期限までに添削が返らない場合、またはコーチが例外返却した場合は、全額を返金します。着手後の利用者都合によるキャンセルはできません。添削返却後の返金は利用規約に定める場合に限ります。") },
    );
  }
  return rows;
}

export type TicketPackConfirmInput = { quantity: number; amountYen: number; validityMonths: number; now?: Date };

// 今お支払いが済んだ場合の有効期限（決済完了の日時から N か月）。実際の期限は決済が完了した日時から数える。
// API（Postgres の interval）と同じく、月末を越える日は月の最終日に寄せる（8/31 + 6か月 = 2/28）。日付は日本時間で数える。
export function ticketExpiryIfPaidNow(validityMonths: number, now: Date = new Date()): Date {
  const jstOffset = 9 * 60 * 60 * 1000;
  const j = new Date(now.getTime() + jstOffset);
  const y = j.getUTCFullYear(), m = j.getUTCMonth() + validityMonths;
  const last = new Date(Date.UTC(y, m + 1, 0)).getUTCDate();
  const shifted = Date.UTC(y, m, Math.min(j.getUTCDate(), last), j.getUTCHours(), j.getUTCMinutes(), j.getUTCSeconds(), j.getUTCMilliseconds());
  return new Date(shifted - jstOffset);
}

export function ticketPackConfirmRows({ quantity, amountYen, validityMonths, now }: TicketPackConfirmInput): ConfirmRow[] {
  return [
    { label: L("内容"), value: L("フォーム添削のチケット{quantity}枚（1枚で動画添削1回）", { quantity }) },
    { label: L("総額"), value: L("{v0}（税込）。{quantity}枚分をまとめて先に支払います", { v0: formatYen(amountYen), quantity }) },
    { label: L("確定のタイミング"), value: L("次の画面でお支払いが済んだ時に購入が確定し、チケットが使えるようになります。") },
    { label: L("有効期限"), value: L("決済が完了した日時から{validityMonths}か月（いまお支払いが済むと {v0} ごろまで）。過ぎると未使用の分は失効し、延長はしません。", { validityMonths, v0: formatDateTime(ticketExpiryIfPaidNow(validityMonths, now).toISOString()) }) },
    { label: L("払戻し"), value: L("購入者の都合による払戻しは原則として行いません。次の場合は払戻しをします（利用規約第6条）。") + numberedRefundExceptions() },
    { label: L("使える人"), value: L("購入した本人だけが使えます。譲ることはできません。") },
  ];
}
