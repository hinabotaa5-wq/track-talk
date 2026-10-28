import type { WorkOrder, WorkOrderTerms } from "@/lib/types";
export type Consultation = {
  id: string; request_key: string; status: "draft" | "awaiting_payment" | "offered" | "in_review" | "answered" | "declined" | "expired" | "cancelled";
  funding: "ticket" | "cash"; price_yen: number; payment_state: "unpaid" | "paid" | "refund_pending" | "refunded";
  minor: boolean;
  // 支払う人。18歳未満のパーソナル指導の相談は保護者、それ以外（18歳未満の添削のあとの相談を含む）は本人。
  payer: "requester" | "guardian";
  question: string; answer: string; video_public_id: string | null; video_url: string | null; answer_video_url: string | null;
  clarify_body: string | null; clarification: string | null; clarify_requested_at: string | null; clarified_at: string | null;
  answered_at: string | null; respond_by: string | null; answer_by: string | null; expires_at: string; created_at: string; purged: boolean; hidden: boolean;
  // フォーム添削の追加相談で、コーチにだけ付く取引条件。引き受ける前は terms、引き受けた後は work_order。
  terms?: WorkOrderTerms; work_order?: WorkOrder;
};
export type ConsultationRoom = {
  consultations: Consultation[]; role: "requester" | "coach" | "guardian"; funding: "ticket" | "cash";
  price_yen: number; can_purchase: boolean; block_reason: string; requires_video: boolean;
};
export const consultationStatus: Record<Consultation["status"], string> = {
  draft: "下書き", awaiting_payment: "お支払い待ち", offered: "コーチの受諾待ち", in_review: "回答待ち", answered: "回答済み", declined: "辞退されました", expired: "期限切れ", cancelled: "取り下げ済み",
};
export function canClarify(c: Consultation, now = Date.now()): boolean {
  return c.status === "answered" && !c.clarify_body && !c.hidden && !c.purged && !!c.answered_at && now < new Date(c.answered_at).getTime() + 7 * 86400000;
}
