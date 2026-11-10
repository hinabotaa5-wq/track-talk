"use client";
import { L } from "@/lib/i18n";

import Link from "next/link";

export default function Error({ reset }: { error: Error & { digest?: string }; reset: () => void }) {
  return (
    <div className="space-y-3">
      <h1 className="text-2xl font-bold">{L("ページを表示できません")}</h1>
      <p className="text-sm text-slate-200">{L("しばらくしてからもう一度試してください。")}</p>
      <button type="button" onClick={reset} className="text-blue-400 transition active:scale-[0.98]">{L("再読み込み")}</button>
      <Link href="/" className="block text-blue-400 transition active:scale-[0.98]">{L("トップへ戻る")}</Link>
    </div>
  );
}
