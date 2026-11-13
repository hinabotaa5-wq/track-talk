"use client";
import { L } from "@/lib/i18n";

import Link from "next/link";
import { GraduationCap } from "lucide-react";

// 一覧の右上に置く「コーチ」ボタン。運営者決定（2026-11-14）で、ログイン状態・応募有無によらず常時出す。
// 未ログインで押した場合は /coach 側のガードでログイン画面へ出る。旧仕様（コーチ本人と応募した人だけに出す）の判定は廃止。
export function CoachSelfButton() {
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
