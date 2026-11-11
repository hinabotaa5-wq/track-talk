import { L } from "@/lib/i18n";
import { formatDateTime, formatYen } from "@/lib/format";
import type { TicketNextExpiry } from "@/lib/coaching-tickets";

// 支払い画面（Stripe）へ進む前の最終確認に出す項目（本体 #569）。
// 分量・総額・確定のタイミング・チケットの有効期限・払戻しの条件を、購入のたびに同じ順で見せる。
export type ConfirmRow = { label: string; value: string };

export type ReviewConfirmInput = {
  method: "card" | "paypay" | "tickets";
  priceYen: number;
  coachName?: string;
  dueAt?: string;
  // チケットで払うとき、確保される1枚の有効期限（いちばん早く失効する分から使う）。旧商品の分は期限なし。
  nextExpiry?: TicketNextExpiry | null;
};

export function reviewConfirmRows({ method, priceYen, coachName, dueAt, nextExpiry }: ReviewConfirmInput): ConfirmRow[] {
  const due = dueAt ? formatDateTime(dueAt) : L("申込みから24時間後");
  const rows: ConfirmRow[] = [
    { label: L("内容"), value: coachName ? L("動画添削1回（動画1本・担当 {coachName}）", { coachName }) : L("動画添削1回（動画1本）") },
  ];
  if (method === "tickets") {
    rows.push(
      { label: L("お支払い"), value: L("チケット1枚（このあとのお支払いはありません）") },
      { label: L("使うチケットの有効期限"), value: nextExpiry ? formatDateTime(nextExpiry.expires_at) : L("期限なし（以前に購入した分）") },
      { label: L("確定のタイミング"), value: L("送った時にチケット1枚を確保し、コーチが返却した時に使用済みになります。") },
      { label: L("戻る場合"), value: L("返却期限（{due}）までに返却がない場合や、コーチが辞退した場合は、確保した1枚を戻します。有効期限を過ぎてから戻る分は失効します。", { due }) },
    );
    return rows;
  }
  rows.push({ label: L("総額"), value: L("{v0}（税込）", { v0: formatYen(priceYen) }) });
  if (method === "card") {
    rows.push(
      { label: L("確定のタイミング"), value: L("次の画面でカードの代金を確保（仮押さえ）し、コーチが返却した時に確定します。") },
      { label: L("取消し・返金"), value: L("返却期限（{due}）までに返却がない場合や、コーチが辞退した場合は、確保を取り消すので代金はかかりません。返却された後の返金は、利用規約に定める場合に限ります。", { due }) },
    );
  } else {
    rows.push(
      { label: L("確定のタイミング"), value: L("次の画面でお支払いが済んだ時に確定します。") },
      { label: L("取消し・返金"), value: L("返却期限（{due}）までに返却がない場合や、コーチが辞退した場合は、全額を返金します。返却された後の返金は、利用規約に定める場合に限ります。", { due }) },
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
    { label: L("内容"), value: L("動画添削のチケット{quantity}枚（1枚で動画添削1回）", { quantity }) },
    { label: L("総額"), value: L("{v0}（税込）。{quantity}枚分をまとめて先に支払います", { v0: formatYen(amountYen), quantity }) },
    { label: L("確定のタイミング"), value: L("次の画面でお支払いが済んだ時に購入が確定し、チケットが使えるようになります。") },
    { label: L("有効期限"), value: L("決済が完了した日時から{validityMonths}か月（いまお支払いが済むと {v0} ごろまで）。過ぎると未使用の分は失効し、延長はしません。", { validityMonths, v0: formatDateTime(ticketExpiryIfPaidNow(validityMonths, now).toISOString()) }) },
    { label: L("払戻し"), value: L("購入後の払い戻しは原則として行いません。運営者が動画添削の提供を終了した場合などに限り、未使用の分を払い戻します（利用規約第6条）。") },
    { label: L("使える人"), value: L("購入した本人だけが使えます。譲ることはできません。") },
  ];
}
