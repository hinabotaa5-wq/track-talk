"use client";

import { L } from "@/lib/i18n";
import { Suspense } from "react";
import { CoachTabs } from "./coach-tabs";


// 依頼する前に、どんなコーチがいるかを見られる。添削でもパーソナル指導でも同じ一覧を使う。
export function CoachesView() {
  return (
    <div className="mx-auto max-w-md space-y-6">
      <div className="space-y-2">
        <h1 className="text-2xl font-bold">{L("コーチ一覧")}</h1>
        <p className="text-sm text-slate-200">{L("上のタブで、動画添削の指名とサポートプランを切り替えられます。")}</p>
      </div>
      <Suspense>
        <CoachTabs />
      </Suspense>
    </div>
  );
}
