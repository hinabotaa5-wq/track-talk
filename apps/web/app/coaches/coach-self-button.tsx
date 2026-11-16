"use client";
import { L } from "@/lib/i18n";

import Link from "next/link";
import { GraduationCap, PersonStanding } from "lucide-react";

// 一覧の右上に置く選手・コーチへの入口。運営者決定（2026-11-14／2026-11-17）で、ログイン状態・応募有無によらず常時出す。
// 文字ボタン2つでは切り替えピルが窮屈になるため、両方ともアイコンだけの円ボタン（44px）にする。
// 未ログインで押した場合は遷移先（/player・/coach）側のガードでログイン画面へ出る。応募状態などの取得はしない。
const circleButtonClass =
  "inline-flex size-11 shrink-0 items-center justify-center rounded-full border border-white/25 bg-white/10 text-slate-100 transition hover:border-blue-400/50 hover:bg-blue-500/15 active:scale-[0.98]";

export function CoachSelfButton() {
  return (
    <div className="flex items-center gap-2">
      <Link href="/player" aria-label={L("選手")} className={circleButtonClass}>
        <PersonStanding className="size-5 shrink-0 text-blue-300" aria-hidden />
      </Link>
      <Link href="/coach" aria-label={L("コーチ")} className={circleButtonClass}>
        <GraduationCap className="size-5 shrink-0 text-blue-300" aria-hidden />
      </Link>
    </div>
  );
}
