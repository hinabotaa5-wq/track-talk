import { coachingEnabled } from "@/lib/features";
import type { Metadata } from "next";
import { notFound } from "next/navigation";

// 保護者の同意ページはリンクを知っている人だけが開く。検索に載せない。
export const metadata: Metadata = {
  title: "保護者の同意 | TrackTalk",
  robots: { index: false, follow: false },
};

// オンラインコーチングを提供するまでは、URL を直接開いても使えないようにする。
export default function GuardianLayout({ children }: { children: React.ReactNode }) {
  if (!coachingEnabled) notFound();
  return children;
}
