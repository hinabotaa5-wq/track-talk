"use client";

import "./globals.css";

export default function GlobalError({ reset }: { error: Error & { digest?: string }; reset: () => void }) {
  return (
    <html lang="ja">
      <body className="mx-auto max-w-5xl px-4 py-6">
        <h1 className="text-2xl font-bold">ページを表示できません</h1>
        <p className="mt-3 text-sm text-muted">しばらくしてからもう一度試してください。</p>
        <button type="button" onClick={reset} className="mt-3 text-accent">
          再読み込み
        </button>
      </body>
    </html>
  );
}
