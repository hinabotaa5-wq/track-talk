"use client";

import Link from "next/link";
import { ChevronRight, Gift } from "lucide-react";
import { L } from "@/lib/i18n";

// コーチ一覧のヘッダー下に置く紹介キャンペーンのバナー（本体 #739、見出しのみの表示は #832）。
// /coaching と /coaches の両ルートで共有する CoachesView の先頭に置き、タブ行より上に表示する。
// バナーには見出しだけを出し、説明は移動先の /referral に置く。背景の黄色は半透明にする。
// ログイン状態・購入実績によらず常時表示し、タップでログイン不要の説明ページ /referral へ移動する。閉じるボタンは付けない。
// 見出しは左右アイコンの間の領域で中央寄せにする（本体 #1043）。アイコンは両端のまま動かさない。
export function ReferralBanner() {
  const headline = L("2人に紹介すると500円分無料！");
  return (
    <Link
      href="/referral"
      aria-label={headline}
      className="flex h-14 items-center gap-2 rounded-xl bg-yellow-400/70 px-3 text-slate-900"
    >
      <Gift className="size-6 shrink-0" aria-hidden />
      <span className="min-w-0 flex-1">
        <span className="block truncate text-center text-[13px] font-semibold min-[390px]:text-base">{headline}</span>
      </span>
      <ChevronRight className="size-5 shrink-0" aria-hidden />
    </Link>
  );
}
