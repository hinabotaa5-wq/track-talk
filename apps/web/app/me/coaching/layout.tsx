import { coachingEnabled } from "@/lib/features";
import { notFound } from "next/navigation";

// オンラインコーチングを提供するまでは、URL を直接開いても使えないようにする。
export default function CoachingSetupLayout({ children }: { children: React.ReactNode }) {
  if (!coachingEnabled) notFound();
  return children;
}
