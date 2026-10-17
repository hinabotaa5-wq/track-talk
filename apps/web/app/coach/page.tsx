import { coachingEnabled } from "@/lib/features";
import { notFound } from "next/navigation";
import { CoachDashboard } from "./coach-dashboard";

// コーチ画面は提供を始めるまで閉じる。応募（/coach/apply）は準備に使うので開けておく。
export default function CoachPage() {
  if (!coachingEnabled) notFound();
  return <CoachDashboard />;
}
