"use client";

import { CoachingGate } from "@/components/coaching-gate";
import Link from "next/link";
import type { ReactNode } from "react";

// コーチ画面の個別画面の枠。/coach へ戻る導線と見出しをそろえる。
export function CoachSubpageShell({ title, children }: { title: string; children: ReactNode }) {
  return (
    <CoachingGate>
      <div className="mx-auto max-w-md space-y-4">
        <Link href="/coach" className="inline-block text-sm text-blue-300 underline">
          ← コーチ画面に戻る
        </Link>
        <h1 className="text-2xl font-bold">{title}</h1>
        {children}
      </div>
    </CoachingGate>
  );
}
