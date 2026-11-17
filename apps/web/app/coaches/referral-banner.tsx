"use client";

import Link from "next/link";
import { ChevronRight, Gift } from "lucide-react";
import { L } from "@/lib/i18n";

// コーチ一覧のヘッダー下に置く紹介キャンペーンのバナー（本体 #739、見出しのみの表示は #832）。
// /coaching と /coaches の両ルートで共有する CoachesView の先頭に置き、タブ行より上に表示する。
// バナーには見出しだけを出し、説明は移動先の /referral に置く。背景の黄色は半透明にする。
// ログイン状態・購入実績によらず常時表示し、タップでログイン不要の説明ページ /referral へ移動する。閉じるボタンは付けない。
export function ReferralBanner() {
  const headline = L("友達2人に紹介すると、1回分(500円)無料！");
  return (
    <Link
      href="/referral"
      aria-label={headline}
      className="flex items-center gap-3 rounded-xl bg-yellow-400/70 p-4 text-slate-900"
    >
      <Gift className="size-6 shrink-0" aria-hidden />
      <span className="min-w-0 flex-1">
        <span className="block font-bold">{headline}</span>
      </span>
      <ChevronRight className="size-5 shrink-0" aria-hidden />
    </Link>
  );
}
