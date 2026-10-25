"use client";

import { useEffect, useRef } from "react";

// LoadMore は、一覧の末尾が画面に近づいたら続きを読む。自動で読めない環境ではボタンで読む。
export function LoadMore({ onLoad, loading, error, expired }: { onLoad: () => void; loading: boolean; error: string; expired: boolean }) {
  const ref = useRef<HTMLButtonElement>(null);
  const load = useRef(onLoad);
  useEffect(() => { load.current = onLoad; });
  useEffect(() => {
    const el = ref.current;
    // 読み終えるたびに見張り直し、まだ末尾が見えていればそのまま次を読む。
    if (!el || error || loading || typeof IntersectionObserver === "undefined") return;
    const observer = new IntersectionObserver((entries) => {
      if (entries.some((entry) => entry.isIntersecting)) load.current();
    }, { rootMargin: "600px 0px" });
    observer.observe(el);
    return () => observer.disconnect();
  }, [error, loading]);
  if (expired) {
    return (
      <button type="button" className="block w-full py-3 text-center text-sm text-slate-300" onClick={() => window.location.reload()}>
        一覧の有効期限が切れました。更新する
      </button>
    );
  }
  return (
    <button
      ref={ref}
      type="button"
      onClick={onLoad}
      disabled={loading}
      aria-live="polite"
      className="block w-full rounded-full border border-white/20 glass bg-white/10 py-3 text-center text-sm font-semibold text-slate-200 shadow-xs transition hover:bg-white/16 hover:text-white active:scale-[0.98] disabled:opacity-60"
    >
      {loading ? "読み込み中…" : error ? "読み込みに失敗しました。再試行" : "続きを読み込む"}
    </button>
  );
}
