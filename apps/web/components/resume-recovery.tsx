"use client";
import { L } from "@/lib/i18n";

import { useEffect, useRef, useState } from "react";
import { usePathname, useRouter, useSearchParams } from "next/navigation";
import { X } from "lucide-react";
import { hidesHeader } from "./bottom-nav";
import { useAuth } from "./auth-provider";
import { profileIncomplete } from "@/lib/profile-gate";
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

// 外部サイト（Stripe の口座登録など）から戻った画面と最初の遷移先で、ヘッダーと本文の描画を確かめる。
// 普段の初期表示やタブ移動では何もしない。壊れていたら、まず表示の取り直し（入力は保たれる）を
// 1回だけ試し、それでも直らなければ再読み込みの導線を出す。文書の再読み込みは利用者の操作に任せ、
// 自動では行わない（往復を防ぐ）。
export function ResumeRecovery() {
  const pathname = usePathname();
  const search = useSearchParams().toString();
  const router = useRouter();
  const { profile } = useAuth();
  const mustSetProfile = profileIncomplete(profile);
  const [offerReload, setOfferReload] = useState(false);
  const routeRef = useRef({ pathname, search, mustSetProfile });
  useEffect(() => {
    routeRef.current = { pathname, search, mustSetProfile };
  }, [pathname, search, mustSetProfile]);
  const episode = useRef({ armed: false, returnPath: null as string | null, refreshed: false, timer: undefined as ReturnType<typeof setTimeout> | undefined });
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
      const expectHeader = !hidesHeader(route.pathname, route.search, route.mustSetProfile);
      const broken = (expectHeader && isHeaderMissing(document)) || isMainEmpty(document);
      if (!broken) {
        state.armed = false;
        setOfferReload(false);
        return;
      }
      if (!state.refreshed && shouldRefreshAgain(window.sessionStorage)) {
        state.refreshed = true;
        router.refresh();
        state.timer = setTimeout(check, recoveryRecheckMs);
        return;
      }
      state.armed = false;
      setOfferReload(true);
    }

    function arm() {
      state.armed = true;
      setOfferReload(false);
      clearTimer();
      state.timer = setTimeout(check, recoveryGraceMs);
    }
    armRef.current = arm;

    function startReturn() {
      state.returnPath = routeRef.current.pathname;
      state.refreshed = false;
      arm();
    }

    function onPageHide() {
      notePageHide(window.sessionStorage);
    }
    function onPageShow(event: PageTransitionEvent) {
      // 戻る・進むで生きた文書が戻ったときも確かめる（再取得が走らない復帰のため）。
      if (event.persisted) startReturn();
    }
    function onVisibility() {
      if (document.hidden) {
        hiddenAt = Date.now();
        return;
      }
      const at = hiddenAt;
      hiddenAt = null;
      if (at !== null && Date.now() - at >= hiddenTripGapMs) startReturn();
    }

    // 戻り画面が正常でも、最初の pathname 変更までは次のチェックを予約しておく。
    if (consumeExternalReturn(window.sessionStorage)) startReturn();
    else if (state.armed) arm(); // Strict Mode の effect 再登録でも予約済みの判定を保つ。
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

  // 外部復帰後の最初の画面遷移だけは、戻り画面のチェック完了後でも構える。
  // クエリだけの更新では予約を消費しない。判定待ちの遷移は猶予を取り直す。
  useEffect(() => {
    const state = episode.current;
    const firstDestination = state.returnPath !== null && state.returnPath !== pathname;
    if (firstDestination) state.returnPath = null;
    if (!state.armed && !firstDestination) return;
    armRef.current();
  }, [pathname, search]);

  if (!offerReload) return null;

  return (
    <div
      role="alert"
      className="fixed inset-x-0 z-50 px-3 bottom-[calc(4.5rem+env(safe-area-inset-bottom))] md:bottom-6"
    >
      <div className="mx-auto flex max-w-md items-center gap-3 rounded-2xl border border-white/25 bg-slate-900/90 p-3 shadow-lg backdrop-blur-xl">
        <p className="min-w-0 flex-1 text-xs leading-snug text-slate-100">{L("画面が正しく表示されませんでした。再読み込みしてください。")}</p>
        <button
          type="button"
          onClick={() => window.location.reload()}
          className="min-h-9 shrink-0 rounded-md bg-blue-600 px-3.5 text-xs font-semibold text-pure-white transition active:scale-[0.98]"
        >{L("再読み込み")}</button>
        <button
          type="button"
          onClick={() => setOfferReload(false)}
          aria-label={L("閉じる")}
          className="flex size-9 shrink-0 items-center justify-center rounded-full text-slate-300 transition hover:bg-white/16 hover:text-white active:scale-[0.98]"
        >
          <X className="size-4" aria-hidden />
        </button>
      </div>
    </div>
  );
}
