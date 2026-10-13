"use client";

import Link from "next/link";

export default function Error({ reset }: { error: Error & { digest?: string }; reset: () => void }) {
  return (
    <div className="space-y-3">
      <h1 className="text-2xl font-bold">ページを表示できません</h1>
      <p className="text-sm text-muted">しばらくしてからもう一度試してください。</p>
      <button type="button" onClick={reset} className="text-accent">
        再読み込み
      </button>
      <Link href="/" className="block text-accent">
        トップへ戻る
      </Link>
    </div>
  );
}
