"use client";

import { useEffect } from "react";
import { usePathname, useSearchParams } from "next/navigation";
import { clearLinkPending, markAppHydrated } from "@/lib/pending-link";

// 画面の URL が変わったら、タップしたリンクのクルクルを止める。
export function PendingLinkReset() {
  const pathname = usePathname();
  const search = useSearchParams().toString();
  useEffect(() => {
    markAppHydrated();
    clearLinkPending();
  }, [pathname, search]);
  return null;
}
