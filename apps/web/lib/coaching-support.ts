import { L } from "./i18n";
export type SupportCall = { id: string; starts_at: string; ends_at: string; join_url: string; status: "proposed" | "confirmed" | "completion_pending" | "completed" | "cancelled" };
export type SupportVideo = { question_id: string; answer_id: string | null; created_at: string; answered_at: string | null };
export type CoachingSupport = {
 role: "coach" | "requester" | "guardian"; minor: boolean; calls: SupportCall[]; videos: SupportVideo[];
 usage: { video_submitted: number; video_answered: number; calls_reserved: number; calls_completed: number };
 total_units: number; delivered_units: number; fulfillment_state: string; refund_amount_yen: number | null;
 call_count: number; call_minutes: number; video_reviews: number;
};
export const callStatusLabels: Record<SupportCall["status"], string> = { get proposed() { return L("候補・参加者の確認待ち"); }, get confirmed() { return L("予約確定"); }, get completion_pending() { return L("実施報告・参加者の確認待ち"); }, get completed() { return L("実施確認済み"); }, get cancelled() { return L("取り消し・回数返却"); } };
