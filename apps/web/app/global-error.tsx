"use client";
import { L } from "@/lib/i18n";

export default function GlobalError({ reset }: { error: Error & { digest?: string }; reset: () => void }) {
  return (
    <html lang="ja">
      <body className="mx-auto min-h-screen max-w-5xl bg-surface px-4 py-6 text-slate-100">
        <h1 className="text-2xl font-bold">{L("ページを表示できません")}</h1>
        <p className="mt-3 text-sm text-slate-200">{L("しばらくしてからもう一度試してください。")}</p>
        <button type="button" onClick={reset} className="mt-3 text-blue-400 transition active:scale-[0.98]">{L("再読み込み")}</button>
      </body>
    </html>
  );
}
