export type TicketProduct = { id: string; title: string; quantity: number; amount_yen: number };
export type TicketPurchase = {
  id: string; request_key: string; quantity: number; amount_yen: number;
  status: "pending" | "paid" | "partially_refunded" | "refunded";
  available: number; reserved: number; refunded_yen: number; created_at: string; refund_pending?: boolean;
};
export type TicketSummary = {
  products: TicketProduct[] | null;
  balance: { available: number; reserved: number };
  purchases: TicketPurchase[] | null;
  ledger: { id: string; kind: string; available_delta: number; reserved_delta: number; created_at: string }[] | null;
  can_purchase: boolean; purchase_block_reason: string; unit_yen: number;
  // false の間はチケットの新規販売を止めている。購入済みの残高・履歴・返金はそのまま。
  sales_open?: boolean;
};

export function ticketReturnPath(value: string | null | undefined): string {
  if (["/coaching/new", "/me/coaching", "/me/coaching/tickets"].includes(value ?? "")) return value!;
  if (/^\/coaching\/requests\/[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}$/.test(value ?? "")) return value!;
  return "/me/coaching/tickets";
}

export const ticketStatusLabels: Record<TicketPurchase["status"], string> = {
  pending: "お支払い待ち", paid: "購入済み", partially_refunded: "一部返金済み", refunded: "返金済み",
};
export const ticketLedgerLabels: Record<string, string> = {
  purchase: "購入", reserve: "依頼分を確保", consume: "添削・相談完了", release: "確保を解除", refund: "返金分を確保", refund_failed: "返金できなかった分を戻しました",
};
