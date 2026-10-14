"use client";

import {
  createContext,
  useCallback,
  useContext,
  useEffect,
  useMemo,
  useRef,
  useState,
  type MouseEvent,
  type ReactNode,
} from "react";
import { useRouter } from "next/navigation";

type LeaveGuardValue = {
  setDirty: (dirty: boolean) => void;
  /** 入力中なら確認ダイアログを出し、確認後に go を実行する。入力中でなければ false を返し何もしない。 */
  confirmLeave: (go: () => void) => boolean;
};

const LeaveGuardContext = createContext<LeaveGuardValue | null>(null);

export function LeaveGuardProvider({ children }: { children: ReactNode }) {
  const dirtyRef = useRef(false);
  const [pending, setPending] = useState<(() => void) | null>(null);

  const setDirty = useCallback((dirty: boolean) => {
    dirtyRef.current = dirty;
  }, []);

  const confirmLeave = useCallback((go: () => void) => {
    if (!dirtyRef.current) return false;
    setPending(() => go);
    return true;
  }, []);

  useEffect(() => {
    function onBeforeUnload(event: BeforeUnloadEvent) {
      if (!dirtyRef.current) return;
      event.preventDefault();
      event.returnValue = "";
    }
    window.addEventListener("beforeunload", onBeforeUnload);
    return () => window.removeEventListener("beforeunload", onBeforeUnload);
  }, []);

  const value = useMemo(() => ({ setDirty, confirmLeave }), [setDirty, confirmLeave]);

  return (
    <LeaveGuardContext.Provider value={value}>
      {children}
      {pending ? (
        <div className="fixed inset-0 z-[100] flex items-center justify-center bg-black/50 px-6" role="presentation">
          <div
            role="alertdialog"
            aria-modal="true"
            aria-labelledby="leave-guard-title"
            aria-describedby="leave-guard-desc"
            className="w-full max-w-sm space-y-4 rounded-2xl border border-white/20 bg-slate-900 p-5 text-white shadow-xl"
          >
            <h2 id="leave-guard-title" className="text-lg font-bold">
              ページを離れますか？
            </h2>
            <p id="leave-guard-desc" className="text-sm text-slate-300">
              ページを離れると入力されたデータが消えます。
            </p>
            <div className="flex flex-col gap-2">
              <button
                type="button"
                autoFocus
                onClick={() => setPending(null)}
                className="h-11 rounded-full bg-blue-500 font-semibold text-white transition active:scale-95"
              >
                入力を続ける
              </button>
              <button
                type="button"
                onClick={() => {
                  const go = pending;
                  dirtyRef.current = false;
                  setPending(null);
                  go();
                }}
                className="h-11 rounded-full border border-white/25 text-slate-200 transition hover:bg-white/10 active:scale-95"
              >
                ページを離れる
              </button>
            </div>
          </div>
        </div>
      ) : null}
    </LeaveGuardContext.Provider>
  );
}

export function useLeaveGuard() {
  const value = useContext(LeaveGuardContext);
  if (!value) throw new Error("LeaveGuardProvider is missing");
  return value;
}

/** リンクのクリックを入力中なら確認ダイアログ経由にする。 */
export function useGuardedLinkClick() {
  const { confirmLeave } = useLeaveGuard();
  const router = useRouter();
  return useCallback(
    (event: MouseEvent, href: string) => {
      if (event.metaKey || event.ctrlKey || event.shiftKey || event.altKey || event.button !== 0) return;
      if (confirmLeave(() => router.push(href))) event.preventDefault();
    },
    [confirmLeave, router],
  );
}

/** 子孫の入力要素に変更があれば離脱警告を有効にし、アンマウント時に解除する。 */
export function LeaveGuardArea({ children, className }: { children: ReactNode; className?: string }) {
  const { setDirty } = useLeaveGuard();

  useEffect(() => () => setDirty(false), [setDirty]);

  return (
    <div className={className} onInput={() => setDirty(true)} onChange={() => setDirty(true)}>
      {children}
    </div>
  );
}
