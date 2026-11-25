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
  // チケットの販売は終了した（本体 #1031、API は常に false）。発行済みの残高・履歴・返金はそのまま。
  sales_open?: boolean;
  // 本体 #1031：紹介特典の ¥500 クーポンの保有状況。古い API では返らない。
  coupons?: CouponSummary | null;
  next_expiry?: TicketNextExpiry | null;
  is_minor?: boolean | null;
  // 単発の料金・添削1回に使う枚数・有効期限（月）。価格の比較と条件の表示に使う。
  single_price_yen?: number; review_units?: number; validity_months?: number;
  // 本体 #687：通常チケット（購入分）と無料チケット（紹介特典）ごとの使える枚数といちばん早い有効期限。
  // balance（合計）と next_expiry（全体の最速）の意味は変えない。古い API では返らない。
  balance_by_kind?: Record<TicketKind, TicketKindBalance> | null;
};

// 紹介特典の ¥500 クーポン（本体 #1031）。使える枚数・いちばん早い有効期限・額面・条件。
export type CouponSummary = { available: number; next_expires_at: string | null; amount_yen: number; terms?: string };

// 使えるクーポンの枚数（サマリが取れない・古い API では 0）。
export function couponCount(summary: Pick<TicketSummary, "coupons"> | null | undefined): number {
  return summary?.coupons?.available ?? 0;
}

// クーポンを使ったときの支払額（0円未満にはしない）。API の couponCashPrice と同じ。
export function couponCashPrice(priceYen: number, couponYen: number): number {
  return Math.max(priceYen - couponYen, 0);
}

// チケットの種別（本体 #687）。normal＝通常チケット（購入分）、reward＝無料チケット（紹介特典）。
export type TicketKind = "normal" | "reward";
export type TicketKindBalance = { available: number; next_expires_at: string | null };

// コーチ詳細の入口から申込画面へ渡す支払いの初期値（?pay=）。coupon は ¥500 クーポン（本体 #1031）。
export type ReviewPayParam = "card" | "coupon" | TicketKind;
export function parseReviewPayParam(value: string | null | undefined): ReviewPayParam | null {
  return value === "card" || value === "coupon" || value === "normal" || value === "reward" ? value : null;
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
// 使える側に直す（チケットが1枚もなければクーポン、クーポンもなければ単発）。
// 指定がなければ、残りがあればチケット、なければクーポン、どちらもなければ単発。
export function initialReviewPayment(
  summary: Pick<TicketSummary, "balance" | "balance_by_kind" | "coupons"> | null | undefined,
  pay: ReviewPayParam | null,
): { method: "card" | "tickets" | "coupon"; kind: TicketKind | null } {
  const counts = ticketKindCounts(summary);
  const kind = defaultTicketKind(counts);
  const total = summary?.balance?.available ?? 0;
  const coupons = couponCount(summary);
  if (pay === "card") return { method: "card", kind };
  if (pay === "coupon" && coupons >= 1) return { method: "coupon", kind };
  if ((pay === "normal" || pay === "reward") && counts && counts[pay] >= 1) return { method: "tickets", kind: pay };
  if (total >= 1) return { method: "tickets", kind };
  return coupons >= 1 ? { method: "coupon", kind } : { method: "card", kind };
}

// 確保される1枚の有効期限（種別を選んだら、その種別の中でいちばん早いもの）。null は期限のない旧ロット。
export function ticketExpiryFor(summary: Pick<TicketSummary, "next_expiry" | "balance_by_kind"> | null | undefined, kind: TicketKind | null): string | null {
  if (kind && summary?.balance_by_kind?.[kind]) return summary.balance_by_kind[kind].next_expires_at ?? null;
  return summary?.next_expiry?.expires_at ?? null;
}

// チケット購入の戻り先。コーチング内の決まった画面だけを許し、それ以外は購入画面に戻す。
// 判定は API の coaching_ticket.go の ticketReturnPath と一致させる（apps/api/internal/httpx/testdata/ticket_return_paths.json で固定）。
const uuidPattern = "[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}";
// 依頼詳細と、コーチ詳細（本体 #691：コーチ詳細の「購入する」と通常チケット0枚の入口から来た購入）。
const ticketReturnPattern = new RegExp(`^/(?:coaching/requests|coaches)/${uuidPattern}$`);
export function ticketReturnPath(value: string | null | undefined): string {
  if (["/coaching/new", "/me/coaching", "/me/coaching/tickets"].includes(value ?? "")) return value!;
  if (ticketReturnPattern.test(value ?? "")) return value!;
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
  // false は終了した周（達成済み、または起点の支払いがなく無効になった周）。
  active: boolean;
  friend_count: number;
  granted_at: string | null;
  expires_at: string | null;
  // 本体 #1031：付与した特典の種類（coupon＝¥500 クーポン、ticket＝発行済みの特典チケット）。古い API では返らない。
  reward_kind?: "coupon" | "ticket" | null;
};
