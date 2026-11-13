"use client";
import { L } from "@/lib/i18n";

import { PageLoading } from "@/components/page-loading";

import { useSearchParams } from "next/navigation";
import { Suspense } from "react";

// 保護者が Stripe の支払い画面から戻ってくるページ。保護者のリンクを Stripe に渡さないよう、共通のページにしている。
function GuardianPaid() {
  const params = useSearchParams();
  const cancelled = params.get("cancelled") === "1";
  const consultation = params.has("consultation");
  return (
    <div className="mx-auto max-w-md space-y-4">
      <h1 className="text-2xl font-bold">{cancelled ? L("お支払いは済んでいません") : L("お支払いを受け付けました")}</h1>
      {cancelled ? (
        <p className="text-sm text-slate-200">{L("保護者のページのリンクをもう一度開き、「お支払いへ進む」から払えます。期限までにお支払いがないと、この申し込みは始まりません。")}</p>
      ) : (
        <p className="text-sm text-slate-200">{L("{v0}やりとりは、最初に受け取った保護者のページのリンクからいつでも見られます。", { v0: consultation ? L("確認が済むと追加質問がコーチに届きます。") : L("確認が済むと期間が始まり、お子さんとコーチに通知が届きます。") })}</p>
      )}
    </div>
  );
}

export default function GuardianPaidPage() {
  return (
    <Suspense fallback={<PageLoading screen label={L("読み込み中...")} />}>
      <GuardianPaid />
    </Suspense>
  );
}
