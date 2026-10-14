"use client";

import { retainExpandedPlayback } from "@/lib/expanded-playback";
import { usePathname } from "next/navigation";
import { useEffect } from "react";

export function ExpandedPlaybackGuard() {
  const pathname = usePathname();
  useEffect(() => {
    retainExpandedPlayback(pathname);
  }, [pathname]);
  return null;
}
