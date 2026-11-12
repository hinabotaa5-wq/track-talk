import { L } from "./i18n";
export type TicketProduct = { id: string; title: string; quantity: number; amount_yen: number };
export type TicketPurchase = {
  id: string; request_key: string; quantity: number; amount_yen: number;
  status: "pending" | "paid" | "partially_refunded" | "refunded";
  available: number; reserved: number; refunded_yen: number; created_at: string; refund_pending?: boolean;
  // 本体 #569：決済完了日時・使った枚数・失効した枚数・有効期限（旧商品の分は期限なしで null）。
  paid_at?: string | null; consumed?: number; expired?: number; expires_at?: string | null;
};
// いちばん早く失効する日時と、その日に失効する未使用の枚数。
export type TicketNextExpiry = { expires_at: string; available: number };
export type TicketSummary = {
  products: TicketProduct[] | null;
  balance: { available: number; reserved: number };
  purchases: TicketPurchase[] | null;
  ledger: { id: string; kind: string; available_delta: number; reserved_delta: number; created_at: string }[] | null;
  can_purchase: boolean; purchase_block_reason: string; unit_yen: number;
  // false の間はチケットの新規販売を止めている。購入済みの残高・履歴・返金はそのまま。
  sales_open?: boolean;
  next_expiry?: TicketNextExpiry | null;
  is_minor?: boolean | null;
  // 単発の料金・添削1回に使う枚数・有効期限（月）。価格の比較と条件の表示に使う。
  single_price_yen?: number; review_units?: number; validity_months?: number;
};

export function ticketReturnPath(value: string | null | undefined): string {
  if (["/coaching/new", "/me/coaching", "/me/coaching/tickets"].includes(value ?? "")) return value!;
  if (/^\/coaching\/requests\/[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}$/.test(value ?? "")) return value!;
  return "/me/coaching/tickets";
}

export const ticketStatusLabels: Record<TicketPurchase["status"], string> = {
  get pending() { return L("お支払い待ち"); }, get paid() { return L("購入済み"); }, get partially_refunded() { return L("一部返金済み"); }, get refunded() { return L("返金済み"); },
};
export const ticketLedgerLabels: Record<string, string> = {
  get purchase() { return L("購入"); }, get reserve() { return L("依頼分を確保"); }, get consume() { return L("添削・相談完了"); }, get release() { return L("確保を解除"); }, get refund() { return L("返金分を確保"); }, get refund_failed() { return L("返金できなかった分を戻しました"); }, get expire() { return L("有効期限切れで失効"); },
};
