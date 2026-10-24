"use client";

import { useEffect, useRef } from "react";
import { usePathname, useRouter, useSearchParams } from "next/navigation";
import { isLastTabStartupRedirecting, readLastTab, rememberLastTab } from "@/lib/last-tab";

export function LastTabRestorer() {
  const pathname = usePathname();
  const search = useSearchParams().toString();
  const router = useRouter();
  const checked = useRef(false);
  const restoring = useRef(false);

  useEffect(() => {
    // head から移動中のホームでは、二重遷移と保存値の上書きを避ける。
    if (isLastTabStartupRedirecting()) return;
    if (!checked.current) {
      checked.current = true;
      // 起動時だけ復元する。アプリ内のホーム操作や直接開いたリンクを優先する。
      if (pathname === "/" && !search && !window.location.hash) {
        const saved = readLastTab();
        if (saved && saved !== "/") {
          restoring.current = true;
          router.replace(saved, { scroll: false });
          return;
        }
      }
    }
    // 復元の完了前にホームで上書きしない。Strict Mode の再実行にも対応する。
    if (restoring.current && pathname === "/" && !search) return;
    restoring.current = false;
    rememberLastTab(pathname, search);
  }, [pathname, search, router]);

  return null;
}
