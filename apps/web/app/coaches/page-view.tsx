"use client";

import { Suspense } from "react";
import { CoachTabs } from "./coach-tabs";


// 依頼する前に、どんなコーチがいるかを見られる。添削でもパーソナル指導でも同じ一覧を使う。
// /coaching と /coaches の両方の一覧でこの画面を共有する（一覧を二重実装しない）。
export function CoachesView() {
  return (
    <div className="mx-auto max-w-md space-y-6">
      <Suspense>
        <CoachTabs />
      </Suspense>
    </div>
  );
}
