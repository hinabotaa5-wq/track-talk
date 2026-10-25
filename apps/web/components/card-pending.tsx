"use client";

import { useLinkStatus } from "next/link";
import { LoaderCircle } from "lucide-react";

// 投稿のカードをタップしてから詳細が開くまで、カードの上にクルクルを重ねる。
// アプリを開いた直後(React が動く前)のタップは、起動スクリプトの data-boot-pending を CSS で見て出す。
export function CardPending() {
  const { pending } = useLinkStatus();
  return (
    <span className={`card-pending pointer-events-none absolute inset-0 z-20 items-center justify-center bg-slate-950/45 ${pending ? "flex" : "hidden"}`}>
      <LoaderCircle className="size-8 animate-spin text-white" aria-hidden />
      {pending ? <span role="status" className="sr-only">投稿を開いています</span> : null}
    </span>
  );
}
