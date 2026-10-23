"use client";

import { useSearchParams } from "next/navigation";
import { Suspense } from "react";

// 保護者が Stripe の支払い画面から戻ってくるページ。保護者のリンクを Stripe に渡さないよう、共通のページにしている。
function GuardianPaid() {
  const params = useSearchParams();
  const cancelled = params.get("cancelled") === "1";
  const consultation = params.has("consultation");
  return (
    <div className="mx-auto max-w-md space-y-4">
      <h1 className="text-2xl font-bold">{cancelled ? "お支払いは済んでいません" : "お支払いを受け付けました"}</h1>
      {cancelled ? (
        <p className="text-sm text-slate-200">
          保護者のページのリンクをもう一度開き、「お支払いへ進む」から払えます。期限までにお支払いがないと、この申し込みは始まりません。
        </p>
      ) : (
        <p className="text-sm text-slate-200">
          {consultation ? "確認が済むと追加相談がコーチに届きます。" : "確認が済むと期間が始まり、お子さんとコーチに通知が届きます。"}やりとりは、最初に受け取った保護者のページのリンクからいつでも見られます。
        </p>
      )}
    </div>
  );
}

export default function GuardianPaidPage() {
  return (
    <Suspense fallback={<p className="text-sm text-slate-200">読み込み中...</p>}>
      <GuardianPaid />
    </Suspense>
  );
}
