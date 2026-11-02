"use client";

import { useEffect } from "react";
import { usePathname } from "next/navigation";
import { applyThemePreference, readThemePreference } from "@/lib/theme";

// 画面を移ると Next.js が theme-color の meta を書き直すので、選んだ表示の色に合わせ直す。
export function ThemeSync() {
  const pathname = usePathname();
  useEffect(() => {
    applyThemePreference(readThemePreference());
  }, [pathname]);
  return null;
}
