"use client";

import { CoachingGate } from "@/components/coaching-gate";
import type { ReactNode } from "react";

// コーチ画面の個別画面の枠。見出しをそろえる。
// /coach へ戻るのはヘッダーの「＜」だけ（header.tsx の goBack が履歴に関係なく /coach へ戻す）。
export function CoachSubpageShell({ title, children }: { title: string; children: ReactNode }) {
  return (
    <CoachingGate>
      <div className="mx-auto max-w-md space-y-4">
        <h1 className="text-2xl font-bold">{title}</h1>
        {children}
      </div>
    </CoachingGate>
  );
}
