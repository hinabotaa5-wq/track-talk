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
  // 本体 #687：通常チケット（購入分）と無料チケット（紹介特典）ごとの使える枚数といちばん早い有効期限。
  // balance（合計）と next_expiry（全体の最速）の意味は変えない。古い API では返らない。
  balance_by_kind?: Record<TicketKind, TicketKindBalance> | null;
};

// チケットの種別（本体 #687）。normal＝通常チケット（購入分）、reward＝無料チケット（紹介特典）。
export type TicketKind = "normal" | "reward";
export type TicketKindBalance = { available: number; next_expires_at: string | null };

// コーチ詳細の入口から申込画面へ渡す支払いの初期値（?pay=）。
export type ReviewPayParam = "card" | TicketKind;
export function parseReviewPayParam(value: string | null | undefined): ReviewPayParam | null {
  return value === "card" || value === "normal" || value === "reward" ? value : null;
}

// 種別ごとの使える枚数。サマリが取れない・古い API で種別がない場合は null（枚数を出さない）。
export function ticketKindCounts(summary: Partial<TicketSummary> | null | undefined): Record<TicketKind, number> | null {
  const byKind = summary?.balance_by_kind;
  if (!byKind?.normal || !byKind?.reward) return null;
  return { normal: byKind.normal.available, reward: byKind.reward.available };
}

// 入口の指定がないときの内訳の初期選択：両方あれば無料チケット、片方だけならその側。
function defaultTicketKind(counts: Record<TicketKind, number> | null): TicketKind | null {
  if (!counts) return null;
  if (counts.reward >= 1) return "reward";
  if (counts.normal >= 1) return "normal";
  return null;
}

// 申込画面の支払い方法と内訳の初期値。入口の指定（?pay=）を優先し、その種別が使えなくなっていたら
// 使える側に直す（チケットが1枚もなければ単発）。指定がなければ従来どおり、残りがあればチケット。
export function initialReviewPayment(
  summary: Pick<TicketSummary, "balance" | "balance_by_kind"> | null | undefined,
  pay: ReviewPayParam | null,
): { method: "card" | "tickets"; kind: TicketKind | null } {
  const counts = ticketKindCounts(summary);
  const kind = defaultTicketKind(counts);
  const total = summary?.balance?.available ?? 0;
  if (pay === "card") return { method: "card", kind };
  if ((pay === "normal" || pay === "reward") && counts && counts[pay] >= 1) return { method: "tickets", kind: pay };
  return total >= 1 ? { method: "tickets", kind } : { method: "card", kind };
}

// 確保される1枚の有効期限（種別を選んだら、その種別の中でいちばん早いもの）。null は期限のない旧ロット。
export function ticketExpiryFor(summary: Pick<TicketSummary, "next_expiry" | "balance_by_kind"> | null | undefined, kind: TicketKind | null): string | null {
  if (kind && summary?.balance_by_kind?.[kind]) return summary.balance_by_kind[kind].next_expires_at ?? null;
  return summary?.next_expiry?.expires_at ?? null;
}

export function ticketReturnPath(value: string | null | undefined): string {
  if (["/coaching/new", "/me/coaching", "/me/coaching/tickets"].includes(value ?? "")) return value!;
  if (/^\/coaching\/requests\/[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}$/.test(value ?? "")) return value!;
  return "/me/coaching/tickets";
}

export const ticketStatusLabels: Record<TicketPurchase["status"], string> = {
  get pending() { return L("お支払い待ち"); }, get paid() { return L("購入済み"); }, get partially_refunded() { return L("一部返金済み"); }, get refunded() { return L("返金済み"); },
};
export const ticketLedgerLabels: Record<string, string> = {
  get referral_grant() { return L("紹介特典"); },
  get purchase() { return L("購入"); }, get reserve() { return L("依頼分を確保"); }, get consume() { return L("添削・相談完了"); }, get release() { return L("確保を解除"); }, get refund() { return L("返金分を確保"); }, get refund_failed() { return L("返金できなかった分を戻しました"); }, get expire() { return L("有効期限切れで失効"); },
};

export type Referral = {
  code: string;
  friend_count: number;
  granted_at: string | null;
  expires_at: string | null;
};
