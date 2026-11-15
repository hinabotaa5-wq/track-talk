"use client";

import Link from "next/link";
import { ChevronRight, Gift } from "lucide-react";
import { L } from "@/lib/i18n";

// コーチ一覧のヘッダー下に置く紹介キャンペーンのバナー（本体 #739）。
// /coaching と /coaches の両ルートで共有する CoachesView の先頭に置き、タブ行より上に表示する。
// ログイン状態・購入実績によらず常時表示し、未ログインのタップは /me 側の既存ガードでログインへ流す。閉じるボタンは付けない。
export function ReferralBanner() {
  const headline = L("友達2人に紹介すると、1回無料！");
  const body = L("1回プランの都度払いやチケットの購入で、紹介IDが発行されます。すでに有効な紹介IDをお持ちの場合や、今回の購入が友達の紹介IDでカウントされた場合、新しいIDは発行されません。あなたのIDで友達2人が購入すると、紹介してくれた方に無料チケットがプレゼントされます。");
  return (
    <Link
      href="/me"
      aria-label={headline}
      className="flex items-center gap-3 rounded-xl bg-yellow-400 p-4 text-slate-900"
    >
      <Gift className="size-6 shrink-0" aria-hidden />
      <span className="min-w-0 flex-1">
        <span className="block font-bold">{headline}</span>
        <span className="block text-sm">{body}</span>
      </span>
      <ChevronRight className="size-5 shrink-0" aria-hidden />
    </Link>
  );
}
