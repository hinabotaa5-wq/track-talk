export type SupportCall = { id: string; starts_at: string; ends_at: string; join_url: string; status: "proposed" | "confirmed" | "completion_pending" | "completed" | "cancelled" };
export type SupportVideo = { question_id: string; answer_id: string | null; created_at: string; answered_at: string | null };
export type CoachingSupport = {
 role: "coach" | "requester" | "guardian"; minor: boolean; calls: SupportCall[]; videos: SupportVideo[];
 usage: { video_submitted: number; video_answered: number; calls_reserved: number; calls_completed: number };
 total_units: number; delivered_units: number; fulfillment_state: string; refund_amount_yen: number | null;
 call_count: number; call_minutes: number; video_reviews: number;
};
export const callStatusLabels: Record<SupportCall["status"], string> = { proposed: "候補・参加者の確認待ち", confirmed: "予約確定", completion_pending: "実施報告・参加者の確認待ち", completed: "実施確認済み", cancelled: "取り消し・回数返却" };
