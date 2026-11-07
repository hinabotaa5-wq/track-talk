import { CoachingGate } from "@/components/coaching-gate";

// 開始予定日の案内はアクセス時の日時で出し分ける。
export const dynamic = "force-dynamic";

export default function NewReviewRequestLayout({ children }: { children: React.ReactNode }) {
  return <CoachingGate announceVideoReviewStart>{children}</CoachingGate>;
}
