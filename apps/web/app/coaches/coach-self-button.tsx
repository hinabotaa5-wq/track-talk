"use client";
import { L } from "@/lib/i18n";

import Link from "next/link";
import { GraduationCap } from "lucide-react";
import { useEffect, useState } from "react";
import { apiFetch } from "@/lib/api";
import type { CoachApplication } from "@/lib/types";
import { useAuth } from "@/components/auth-provider";

// 一覧の右上に置く「コーチ」ボタン。コーチ本人と応募した人だけに出す（status は問わない）。
// HeaderCoachLink（承認済みだけが対象）とは条件が違うので、流用せずここで判定する。
export function CoachSelfButton() {
  const { token } = useAuth();
  const [result, setResult] = useState<{ token: string; applied: boolean } | null>(null);

  useEffect(() => {
    if (!token) return;
    let cancelled = false;
    apiFetch<{ application: CoachApplication | null }>("/v1/coach/me", { token })
      .then(({ application }) => {
        if (!cancelled) setResult({ token, applied: application !== null });
      })
      .catch(() => {
        if (!cancelled) setResult({ token, applied: false });
      });
    return () => {
      cancelled = true;
    };
  }, [token]);

  // 未ログイン・応募なし・読み込み中・取得失敗の間は出さない。判定前の一瞬表示もしない。
  if (!token || result?.token !== token || !result.applied) return null;
  return (
    <Link
      href="/coach"
      className="inline-flex h-10 shrink-0 items-center gap-1.5 rounded-full border border-white/25 bg-white/10 px-4 text-sm font-semibold text-slate-100 transition hover:border-blue-400/50 hover:bg-blue-500/15 active:scale-[0.98]"
    >
      <GraduationCap className="size-4 shrink-0 text-blue-300" aria-hidden />
      {L("コーチ")}
    </Link>
  );
}
