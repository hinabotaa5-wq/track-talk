"use client";

import { Suspense } from "react";
import { CoachTabs } from "./coach-tabs";


// 依頼する前に、どんなコーチがいるかを見られる。添削でもパーソナル指導でも同じ一覧を使う。
// 見出しと説明文は置かない（本体 #641）。/coaching の着地もこの画面を共有する。
export function CoachesView() {
  return (
    <div className="mx-auto max-w-md space-y-6">
      <Suspense>
        <CoachTabs />
      </Suspense>
    </div>
  );
}
