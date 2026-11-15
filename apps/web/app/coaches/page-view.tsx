"use client";

import { Suspense, useState } from "react";
import { CoachTabs } from "./coach-tabs";
import { ReferralBanner } from "./referral-banner";


// 依頼する前に、どんなコーチがいるかを見られる。添削でもパーソナル指導でも同じ一覧を使う。
// /coaching と /coaches の両方の一覧でこの画面を共有する（一覧を二重実装しない）。
// 初回読み込み中はページ中身をストップウォッチだけにする（本体 #804）。
// 一覧の取得自体はタブ側で行い、終わるまでバナーは出さない。取得中の表示は一覧側の PageLoading screen。
export function CoachesView() {
  const [loading, setLoading] = useState(true);
  return (
    <div className="mx-auto max-w-md space-y-6">
      {loading ? null : <ReferralBanner />}
      <Suspense>
        <CoachTabs onLoadingChange={setLoading} />
      </Suspense>
    </div>
  );
}
