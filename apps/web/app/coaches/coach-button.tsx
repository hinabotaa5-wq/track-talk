"use client";
import { L } from "@/lib/i18n";

import Link from "next/link";
import { GraduationCap } from "lucide-react";
import { useEffect, useState } from "react";
import { apiFetch } from "@/lib/api";
import type { CoachApplication } from "@/lib/types";
import { useAuth } from "@/components/auth-provider";

// 一覧の右上に置くコーチ専用画面への入口。文言は正確に「コーチ」。
// 出す条件は、ログイン済みかつ GET /v1/coach/me の application が null でないこと
// （status は問わない。承認済みだけの HeaderCoachLink とは条件が違う）。
// 読み込み中・取得失敗・未ログイン・応募なしの間は何も出さない。
export function CoachButton() {
  const { token } = useAuth();
  const [result, setResult] = useState<{ token: string; hasApplication: boolean } | null>(null);

  useEffect(() => {
    if (!token) return;
    let cancelled = false;
    apiFetch<{ application: CoachApplication | null }>("/v1/coach/me", { token })
      .then(({ application }) => {
        if (!cancelled) setResult({ token, hasApplication: application != null });
      })
      .catch(() => {
        if (!cancelled) setResult({ token, hasApplication: false });
      });
    return () => {
      cancelled = true;
    };
  }, [token]);

  if (!token || result?.token !== token || !result.hasApplication) return null;
  return (
    <Link
      href="/coach"
      aria-label={L("コーチ")}
      className="inline-flex h-10 shrink-0 items-center gap-1.5 rounded-full border border-white/25 bg-white/10 px-4 text-sm font-semibold text-slate-100 transition hover:border-blue-400/50 hover:bg-blue-500/15 active:scale-[0.98]"
    >
      <GraduationCap className="size-4 shrink-0 text-blue-300" aria-hidden />
      {L("コーチ")}
    </Link>
  );
}
