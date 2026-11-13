"use client";

import { L } from "@/lib/i18n";
import Link from "next/link";


// 退会が終わったあとに飛ぶページ。ログアウト済みなので、ログインなしで見られる場所に置く。
export function GoodbyeView() {
  return (
    <div className="mx-auto max-w-md space-y-4">
      <h1 className="text-2xl font-bold">{L("退会しました")}</h1>
      <p className="text-sm text-slate-300">{L("これまで TrackTalk を使っていただき、ありがとうございました。アカウントとデータは削除されました。")}</p>
      <Link
        href="/"
        className="inline-flex w-full items-center justify-center rounded-full bg-blue-600 px-4.5 py-2 text-sm font-semibold text-pure-white shadow-xs transition-all duration-150 hover:bg-blue-500 active:scale-[0.98] active:bg-blue-700"
      >{L("フィードへ")}</Link>
    </div>
  );
}
