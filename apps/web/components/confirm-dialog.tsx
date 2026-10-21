"use client";

import { createContext, useCallback, useContext, useEffect, useMemo, useRef, useState, type ReactNode } from "react";

type ConfirmOptions = {
  title: string;
  /** 補足の説明。なければ出さない。 */
  body?: string;
  confirmLabel: string;
  cancelLabel?: string;
};

type ConfirmFn = (options: ConfirmOptions) => Promise<boolean>;

const ConfirmContext = createContext<ConfirmFn | null>(null);

// ブラウザ標準の confirm() の代わりに、アプリの見た目に合わせた確認ダイアログを出す。
export function ConfirmProvider({ children }: { children: ReactNode }) {
  const [open, setOpen] = useState<ConfirmOptions | null>(null);
  const resolver = useRef<((ok: boolean) => void) | null>(null);

  const confirm = useCallback<ConfirmFn>((options) => {
    // 開いたままもう一度呼ばれたら、前の問いは「いいえ」で閉じる。
    resolver.current?.(false);
    setOpen(options);
    return new Promise<boolean>((resolve) => {
      resolver.current = resolve;
    });
  }, []);

  const answer = useCallback((ok: boolean) => {
    resolver.current?.(ok);
    resolver.current = null;
    setOpen(null);
  }, []);

  useEffect(() => {
    if (!open) return;
    function onKey(event: KeyboardEvent) {
      if (event.key === "Escape") answer(false);
    }
    window.addEventListener("keydown", onKey);
    return () => window.removeEventListener("keydown", onKey);
  }, [open, answer]);

  const value = useMemo(() => confirm, [confirm]);

  return (
    <ConfirmContext.Provider value={value}>
      {children}
      {open ? (
        <div
          className="fixed inset-0 z-[100] flex items-center justify-center bg-black/50 px-6"
          role="presentation"
          onClick={(event) => {
            if (event.target === event.currentTarget) answer(false);
          }}
        >
          <div
            role="alertdialog"
            aria-modal="true"
            aria-labelledby="confirm-dialog-title"
            aria-describedby={open.body ? "confirm-dialog-desc" : undefined}
            className="w-full max-w-sm space-y-4 rounded-2xl border border-white/20 bg-slate-900 p-5 text-white shadow-xl"
          >
            <h2 id="confirm-dialog-title" className="text-lg font-bold">
              {open.title}
            </h2>
            {open.body ? (
              <p id="confirm-dialog-desc" className="text-sm text-slate-300">
                {open.body}
              </p>
            ) : null}
            <div className="flex flex-col gap-2">
              <button
                type="button"
                onClick={() => answer(true)}
                className="h-11 rounded-full bg-blue-500 font-semibold text-white transition active:scale-95"
              >
                {open.confirmLabel}
              </button>
              <button
                type="button"
                autoFocus
                onClick={() => answer(false)}
                className="h-11 rounded-full border border-white/25 text-slate-200 transition hover:bg-white/10 active:scale-95"
              >
                {open.cancelLabel ?? "やめる"}
              </button>
            </div>
          </div>
        </div>
      ) : null}
    </ConfirmContext.Provider>
  );
}

export function useConfirm() {
  const value = useContext(ConfirmContext);
  if (!value) throw new Error("ConfirmProvider is missing");
  return value;
}

// 通報の確認。どの画面でも同じ文言と見た目にする。
export function reportConfirmOptions(options: { title?: string; body?: string } = {}): ConfirmOptions {
  return {
    title: options.title ?? "この内容を通報しますか？",
    body: options.body ?? "運営が内容を確認します。",
    confirmLabel: "通報する",
  };
}
