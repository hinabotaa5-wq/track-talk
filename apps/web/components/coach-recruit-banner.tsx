"use client";

import Link from "next/link";
import { useSyncExternalStore } from "react";
import { X } from "lucide-react";
import { coachApplyPath } from "@/lib/coach-recruit";

const storageKey = "tracktalk-coach-recruit-dismissed";

function readDismissed() {
  try {
    return localStorage.getItem(storageKey) === "1";
  } catch {
    return false;
  }
}

// localStorage に保存できなくても、閉じたあとはこの画面を開いている間は出さない。
let sessionDismissed = false;
const listeners = new Set<() => void>();

function subscribe(listener: () => void) {
  listeners.add(listener);
  return () => {
    listeners.delete(listener);
  };
}

function saveDismissed() {
  try {
    localStorage.setItem(storageKey, "1");
  } catch {
    // 保存できない環境では、次回も告知を出すだけ。
  }
  sessionDismissed = true;
  listeners.forEach((listener) => listener());
}


// オンラインコーチングを始める前に、コーチだけ先に募集する告知。機能の中身はまだサイトに置かない。
export function CoachRecruitBanner() {
  // サーバーと最初の描画では出さず、閉じたかどうかを読めてから出す。
  const dismissed = useSyncExternalStore(subscribe, () => sessionDismissed || readDismissed(), () => true);

  if (dismissed) return null;

  return (
    <aside
      aria-labelledby="coach-recruit-title"
      className="relative flex items-center gap-3 rounded-2xl border border-blue-400/40 glass bg-blue-500/15 p-3 pr-11"
    >
      <div className="min-w-0 flex-1 space-y-0.5">
        <p className="text-[11px] font-semibold tracking-wide text-blue-300">準備中 · オンラインコーチング</p>
        <h2 id="coach-recruit-title" className="text-sm font-bold text-white">
          コーチを募集しています
        </h2>
        <p className="text-xs leading-snug text-slate-200">
          動画の添削やオンライン指導をしてくれる方を探しています。YouTube や SNS で教えている方も歓迎です。
        </p>
      </div>
      <Link
        href={coachApplyPath}
        className="inline-flex min-h-9 shrink-0 items-center rounded-md bg-blue-600 px-3.5 text-xs font-semibold text-white hover:bg-blue-500"
      >
        応募する
      </Link>
      <button
        type="button"
        aria-label="告知を閉じる"
        onClick={saveDismissed}
        className="absolute top-1.5 right-1.5 flex size-8 cursor-pointer items-center justify-center rounded-full text-slate-300 hover:bg-white/16 hover:text-white"
      >
        <X className="size-4" aria-hidden />
      </button>
    </aside>
  );
}
