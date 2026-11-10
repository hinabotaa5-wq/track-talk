"use client";

import { L } from "@/lib/i18n";
import Link from "next/link";

export default function NotFound() {
  return (
    <div className="space-y-3">
      <h1 className="text-2xl font-bold">{L("ページが見つかりません")}</h1>
      <Link href="/" className="inline-block text-blue-400 transition active:scale-[0.98]">{L("トップへ戻る")}</Link>
    </div>
  );
}
