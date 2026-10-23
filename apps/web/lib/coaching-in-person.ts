export type InPersonSession = { id: string; starts_at: string; ends_at: string; venue_name: string; venue_area: string; meeting_note: string; participants: number; status: "proposed" | "confirmed" | "completion_pending" | "completed" | "cancelled" };
export type CoachingInPerson = {
  role: "coach" | "requester" | "guardian"; minor: boolean; sessions: InPersonSession[];
  usage: { reserved: number; completed: number };
  total_units: number; delivered_units: number; fulfillment_state: string; refund_amount_yen: number | null;
  venue_area: string | null; venue_name: string | null; session_count: number; session_minutes: number; travel_fee_yen: number; max_participants: number;
};
export const inPersonStatusLabels: Record<InPersonSession["status"], string> = { proposed: "候補・参加者の確認待ち", confirmed: "予約確定", completion_pending: "実施報告・参加者の確認待ち", completed: "実施確認済み", cancelled: "取り消し・回数返却" };
