"use client";

import { PageLoading } from "@/components/page-loading";
import { useRouter } from "next/navigation";
import { useEffect } from "react";

// 旧URL（/coach/requests#offers など）の受け皿。ブックマークや古いリンクを新しい画面へ送る。
// 表示そのものは /coach/offers と /coach/reviewing が持つ。
export default function CoachRequestsRedirectPage() {
  const router = useRouter();

  useEffect(() => {
    router.replace(window.location.hash === "#reviewing" ? "/coach/reviewing" : "/coach/offers");
  }, [router]);

  return <PageLoading screen label="読み込み中..." />;
}
