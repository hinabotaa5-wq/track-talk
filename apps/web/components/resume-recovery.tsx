"use client";

import { useEffect, useRef, useState } from "react";
import { usePathname, useRouter, useSearchParams } from "next/navigation";
import { X } from "lucide-react";
import { hidesHeader } from "./bottom-nav";
import {
  consumeExternalReturn,
  hiddenTripGapMs,
  isHeaderMissing,
  isMainEmpty,
  notePageHide,
  recoveryGraceMs,
  recoveryRecheckMs,
  shouldRefreshAgain,
} from "@/lib/resume-recovery";

// 外部サイト（Stripe の口座登録など）から戻った直後だけ、ヘッダーと本文の描画を確かめる。
// 普段の初期表示やタブ移動では何もしない。壊れていたら、まず表示の取り直し（入力は保たれる）を
// 1回だけ試し、それでも直らなければ再読み込みの導線を出す。文書の再読み込みは利用者の操作に任せ、
// 自動では行わない（往復を防ぐ）。
export function ResumeRecovery() {
  const pathname = usePathname();
  const search = useSearchParams().toString();
  const router = useRouter();
  const [offerReload, setOfferReload] = useState(false);
  const routeRef = useRef({ pathname, search });
  useEffect(() => {
    routeRef.current = { pathname, search };
  }, [pathname, search]);
  const episode = useRef({ armed: false, done: false, refreshed: false, timer: undefined as ReturnType<typeof setTimeout> | undefined });
  const armRef = useRef(() => {});

  useEffect(() => {
    const state = episode.current;
    let hiddenAt: number | null = null;

    function clearTimer() {
      clearTimeout(state.timer);
      state.timer = undefined;
    }

    function check() {
      state.timer = undefined;
      // 見えていない間の判定はしない。見えたら非表示往復の経路で構え直す。
      if (document.hidden) return;
      const route = routeRef.current;
      const expectHeader = !hidesHeader(route.pathname, route.search);
      const broken = (expectHeader && isHeaderMissing(document)) || isMainEmpty(document);
      if (!broken) {
        state.armed = false;
        state.done = true;
        state.refreshed = false;
        setOfferReload(false);
        return;
      }
      if (!state.refreshed && shouldRefreshAgain(window.sessionStorage)) {
        state.refreshed = true;
        router.refresh();
        state.timer = setTimeout(check, recoveryRecheckMs);
        return;
      }
      state.done = true;
      setOfferReload(true);
    }

    function arm() {
      // 直った・導線を出した往復は蒸し返さない。
      if (state.done) return;
      state.armed = true;
      setOfferReload(false);
      clearTimer();
      state.timer = setTimeout(check, recoveryGraceMs);
    }
    armRef.current = arm;

    function onPageHide() {
      notePageHide(window.sessionStorage);
    }
    function onPageShow(event: PageTransitionEvent) {
      clearTimer();
      // 戻る・進むで生きた文書が戻ったときも確かめる（再取得が走らない復帰のため）。
      if (event.persisted) arm();
    }
    function onVisibility() {
      if (document.hidden) {
        hiddenAt = Date.now();
        return;
      }
      const at = hiddenAt;
      hiddenAt = null;
      if (at !== null && Date.now() - at >= hiddenTripGapMs) arm();
    }

    // 外部サイトへの往復のあとの読み込みは、ここで構える。以降のタブ移動では構えない。
    if (consumeExternalReturn(window.sessionStorage)) arm();
    window.addEventListener("pagehide", onPageHide);
    window.addEventListener("pageshow", onPageShow);
    document.addEventListener("visibilitychange", onVisibility);
    return () => {
      clearTimer();
      window.removeEventListener("pagehide", onPageHide);
      window.removeEventListener("pageshow", onPageShow);
      document.removeEventListener("visibilitychange", onVisibility);
    };
  }, [router]);

  // 構えている間にタブを移ったら、移り先でも確かめ直す。
  useEffect(() => {
    const state = episode.current;
    if (!state.armed || state.done) return;
    armRef.current();
  }, [pathname, search]);

  if (!offerReload) return null;

  return (
    <div
      role="alert"
      className="fixed inset-x-0 z-50 px-3 bottom-[calc(4.5rem+env(safe-area-inset-bottom))] md:bottom-6"
    >
      <div className="mx-auto flex max-w-md items-center gap-3 rounded-2xl border border-white/25 bg-slate-900/90 p-3 shadow-lg backdrop-blur-xl">
        <p className="min-w-0 flex-1 text-xs leading-snug text-slate-100">
          画面が正しく表示されませんでした。再読み込みしてください。
        </p>
        <button
          type="button"
          onClick={() => window.location.reload()}
          className="min-h-9 shrink-0 rounded-md bg-blue-600 px-3.5 text-xs font-semibold text-pure-white transition active:scale-[0.98]"
        >
          再読み込み
        </button>
        <button
          type="button"
          onClick={() => setOfferReload(false)}
          aria-label="閉じる"
          className="flex size-9 shrink-0 items-center justify-center rounded-full text-slate-300 transition hover:bg-white/16 hover:text-white active:scale-[0.98]"
        >
          <X className="size-4" aria-hidden />
        </button>
      </div>
    </div>
  );
}
