import { coachingEnabled } from "@/lib/features";
import { notFound } from "next/navigation";

// オンラインコーチングを提供するまでは、URL を直接開いても 404 にする。
// 閉じる画面の layout から使う。API 側の COACHING_ENABLED と揃えること。
export function CoachingGate({ children }: { children: React.ReactNode }) {
  if (!coachingEnabled) notFound();
  return children;
}
