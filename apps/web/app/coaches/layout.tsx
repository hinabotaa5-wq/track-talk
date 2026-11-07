import { CoachingGate } from "@/components/coaching-gate";
import { coachingCoachListEnabled, coachingEnabled } from "@/lib/features";

// コーチ一覧とコーチの詳細は、コーチングを開けたうえで一覧のフラグも立っているときだけ見せる。
// どちらかが閉じている間は、ほかの閉じた画面と同じ準備中を出す。
export default function CoachesLayout({ children }: { children: React.ReactNode }) {
  return <CoachingGate open={coachingEnabled && coachingCoachListEnabled}>{children}</CoachingGate>;
}
