"use client";

import { L } from "@/lib/i18n";
import { coachingEnabled } from "@/lib/features";
import { videoReviewStartNotice } from "@/lib/video-review-start";
import { CoachApplyLink } from "./coach-apply-link";

// 提供前は直接URLを開いても申込み画面を出さず、募集だけを案内する。
// 閉じる画面の layout から使う。API 側の COACHING_ENABLED と揃えること。
// open を渡すと、コーチングを開けていてもそれが偽の間は同じ準備中を出す（コーチ一覧など）。
export function CoachingGate({
  children,
  open = coachingEnabled,
  announceVideoReviewStart = false,
}: {
  children: React.ReactNode;
  open?: boolean;
  announceVideoReviewStart?: boolean;
}) {
  if (open) return children;

  const startNotice = announceVideoReviewStart ? videoReviewStartNotice() : null;
  return (
    <div className="mx-auto max-w-md space-y-6">
      <div className="space-y-3">
        <h1 className="text-2xl font-bold">{L("ただいま準備中です")}</h1>
        <p className="text-sm leading-relaxed text-slate-200">{L("1回プランと継続プランは準備中です。いまは指導の申込み・チケットの購入を受け付けていません。現在は、選手を支えるコーチを募集しています。")}</p>
        {startNotice && <p className="text-sm leading-relaxed text-slate-200">{startNotice}</p>}
      </div>
      <CoachApplyLink />
    </div>
  );
}
