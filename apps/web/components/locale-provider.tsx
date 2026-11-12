"use client";

// 表示言語の決め方（運営者決定 2026-11-13）：ログイン状態を問わず、ブラウザ言語の自動判定だけで決める。
// 端末保存・プロフィール locale・過去の手動選択は表示言語に使わない（残っていても無視する）。
// 決まった言語は、画面全体（ヘッダー・下メニューを含む）を key で描き直して即時に反映する。

import { createContext, Fragment, useContext, useEffect, useMemo, useState, useSyncExternalStore, type ReactNode } from "react";
import { usePathname } from "next/navigation";
import {
  detectBrowserLocale,
  englishLoaded,
  loadEnglish,
  localePendingClass,
  registerTitlePart,
  setActiveLocale,
  titleChangedEvent,
  translateTitle,
  type Locale,
} from "@/lib/i18n";
import { applyZodLocale } from "@/lib/validators";

const LocaleContext = createContext<Locale>("ja");

function subscribeBrowserLanguage(onChange: () => void) {
  window.addEventListener("languagechange", onChange);
  return () => {
    window.removeEventListener("languagechange", onChange);
  };
}

// サーバー描画とハイドレーションの最初の描画は日本語。ブラウザ言語は描画後に当てる。
function useBrowserLocale(): Locale {
  return useSyncExternalStore(subscribeBrowserLanguage, detectBrowserLocale, () => "ja");
}

export function LocaleProvider({ children }: { children: ReactNode }) {
  const browser = useBrowserLocale();
  const pathname = usePathname();
  const wanted: Locale = browser;
  const [applied, setApplied] = useState<Locale>("ja");

  useEffect(() => {
    let cancelled = false;
    if (wanted === "en" && !englishLoaded()) {
      loadEnglish()
        .then(() => {
          if (!cancelled) setApplied("en");
        })
        .catch(() => {
          // 辞書を読めなければ日本語のまま出す（画面を隠し続けない）。
          if (!cancelled) setApplied("ja");
        });
    } else {
      // eslint-disable-next-line react-hooks/set-state-in-effect
      setApplied(wanted);
    }
    return () => {
      cancelled = true;
    };
  }, [wanted]);

  // 描画の前に決める（子の L が新しい言語で引けるように）。
  setActiveLocale(applied);

  useEffect(() => {
    applyZodLocale(applied);
    document.documentElement.lang = applied;
    // 起動時に英語の人だけ隠していた画面を見せる（lib/i18n の localeStartupScript）。
    if (applied === wanted) document.documentElement.classList.remove(localePendingClass);
  }, [applied, wanted]);

  // タブのタイトル（metadata の日本語）も英語にする。「ページ名｜TrackTalk」の形を部分ごとに引く。
  useEffect(() => {
    const titleElement = document.querySelector("title");
    const translate = () => {
      if (applied !== "en") return;
      const next = translateTitle(document.title);
      if (next !== document.title) document.title = next;
    };
    translate();
    window.addEventListener(titleChangedEvent, translate);
    if (!titleElement) return () => window.removeEventListener(titleChangedEvent, translate);
    const observer = new MutationObserver(translate);
    observer.observe(titleElement, { childList: true, characterData: true, subtree: true });
    return () => {
      observer.disconnect();
      window.removeEventListener(titleChangedEvent, translate);
    };
  }, [applied, pathname]);

  const value = useMemo(() => applied, [applied]);

  return (
    <LocaleContext.Provider value={value}>
      {/* 言語が変わったら中身を描き直す。L はこの描き直しで新しい言語を引く。 */}
      <Fragment key={applied}>{children}</Fragment>
    </LocaleContext.Provider>
  );
}

export function useLocale(): Locale {
  return useContext(LocaleContext);
}

// サーバーで日本語で作ったタイトルの部分（ja）に、今の表示言語での文言（translated）を対応づける。
export function useTitleTranslation(ja: string, translated: string) {
  useEffect(() => {
    registerTitlePart(ja, translated);
  }, [ja, translated]);
}
