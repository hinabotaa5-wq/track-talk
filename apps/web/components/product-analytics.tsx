"use client";

import { useEffect, useRef } from "react";
import { usePathname } from "next/navigation";
import { capturePosthog } from "@/lib/posthog";

export function ProductAnalytics() {
  const pathname = usePathname();
  const previous = useRef<string | null>(null);
  useEffect(() => {
    if (!pathname || previous.current === pathname) return;
    previous.current = pathname;
    capturePosthog("$pageview", pathname);
  }, [pathname]);
  return null;
}
