"use client";

import Link from "next/link";
import { GraduationCap } from "lucide-react";
import { usePathname } from "next/navigation";
import { useEffect, useState } from "react";
import { apiFetch } from "@/lib/api";
import type { CoachApplication } from "@/lib/types";
import { useAuth } from "./auth-provider";
import { useGuardedLinkClick } from "./leave-guard";

export function HeaderCoachLink() {
  const { token } = useAuth();
  const pathname = usePathname();
  const guardedClick = useGuardedLinkClick();
  const [result, setResult] = useState<{ token: string; approved: boolean } | null>(null);

  useEffect(() => {
    if (!token) return;
    let cancelled = false;
    const refresh = () => {
      if (document.visibilityState === "hidden") return;
      apiFetch<{ application: CoachApplication | null }>("/v1/coach/me", { token })
        .then(({ application }) => {
          if (!cancelled) setResult({ token, approved: application?.status === "approved" });
        })
        .catch(() => { if (!cancelled) setResult({ token, approved: false }); });
    };
    refresh();
    window.addEventListener("focus", refresh);
    document.addEventListener("visibilitychange", refresh);
    return () => {
      cancelled = true;
      window.removeEventListener("focus", refresh);
      document.removeEventListener("visibilitychange", refresh);
    };
  }, [token, pathname]);

  if (!token || result?.token !== token || !result.approved) return null;
  return (
    <Link href="/coach" aria-label="コーチ専用画面" title="コーチ専用画面"
      onClick={(event) => guardedClick(event, "/coach")}
      className="inline-flex size-10 items-center justify-center rounded-full text-blue-300 transition hover:bg-white/16 active:scale-[0.98]">
      <GraduationCap className="size-5" aria-hidden />
    </Link>
  );
}
