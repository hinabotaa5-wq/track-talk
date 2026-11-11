"use client";

// 表示言語の決め方と切り替え（本体 #530・#567）。
// - ログイン済みでニックネーム・種目を決め終えた人：アカウントの設定（profiles.locale）。
//   ただしログイン直前の端末での明示選択（プロンプト・登録画面・ログイン画面で選んだもの）があり保存値と
//   違う場合は、その選択を勝たせてアカウントにも保存する（本体 #567）。明示選択が無ければ保存値のまま。
// - それ以外（未ログイン・登録直後のオンボーディング中）：端末に保存した言語
// - マイページで選び直した直後（保存前）：その選択（preview）。画面を移ると取り消す
// 決まった言語は、画面全体（ヘッダー・下メニューを含む）を key で描き直して即時に反映する。

import { createContext, Fragment, useCallback, useContext, useEffect, useMemo, useRef, useState, useSyncExternalStore, type ReactNode } from "react";
import { usePathname } from "next/navigation";
import { useAuth } from "./auth-provider";
import { profileIncomplete } from "@/lib/profile-gate";
import { apiFetch } from "@/lib/api";
import {
  clearLocaleExplicitChoice,
  englishLoaded,
  hasLocaleExplicitChoice,
  loadEnglish,
  localeChangedEvent,
  localePendingClass,
  markLocaleExplicitChoice,
  notifyLocaleChanged,
  parseLocale,
  readStoredLocale,
  registerTitlePart,
  setActiveLocale,
  storeLocale,
  titleChangedEvent,
  translateTitle,
  type Locale,
} from "@/lib/i18n";
import { applyZodLocale } from "@/lib/validators";

type LocaleState = {
  locale: Locale;
  // 端末にも保存して即時に切り替える（未ログイン時の選択）。
  chooseDeviceLocale: (locale: Locale) => void;
  // 保存前の試し表示（マイページの言語の選択）。null で取り消す。
  previewLocale: (locale: Locale | null) => void;
  preview: Locale | null;
};

const LocaleContext = createContext<LocaleState>({
  locale: "ja",
  chooseDeviceLocale: (next) => {
    storeLocale(next);
    markLocaleExplicitChoice();
    notifyLocaleChanged();
  },
  previewLocale: () => {},
  preview: null,
});

function subscribeDevice(onChange: () => void) {
  window.addEventListener(localeChangedEvent, onChange);
  window.addEventListener("storage", onChange);
  return () => {
    window.removeEventListener(localeChangedEvent, onChange);
    window.removeEventListener("storage", onChange);
  };
}

// サーバー描画とハイドレーションの最初の描画は日本語。端末の言語は描画後に当てる。
function useDeviceLocale(): Locale {
  return useSyncExternalStore(subscribeDevice, readStoredLocale, () => "ja");
}

export function accountLocale(session: unknown, profile: { locale?: string | null; nickname?: string | null; event?: string | null } | null): Locale | null {
  if (!session || !profile) return null;
  // 登録直後（オンボーディング中）のアカウントは DB の既定値 ja なので、端末で選んだ言語を優先する。
  if (profileIncomplete(profile as never)) return null;
  return parseLocale(profile.locale);
}

export function LocaleProvider({ children }: { children: ReactNode }) {
  const { session, profile, token, refreshProfile } = useAuth();
  const device = useDeviceLocale();
  const pathname = usePathname();
  const [preview, setPreview] = useState<{ locale: Locale; path: string } | null>(null);
  // ログイン時の引き継ぎ（本体 #567）：明示選択と保存値が違うあいだ、保存が終わるまで選んだ言語で出し続ける。
  const [handoff, setHandoff] = useState<Locale | null>(null);
  const handoffAttempt = useRef<string | null>(null);
  const account = accountLocale(session, profile);
  const activePreview = preview && preview.path === pathname ? preview.locale : null;
  const wanted: Locale = activePreview ?? handoff ?? account ?? device;
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

  // ログイン時の言語の引き継ぎ（本体 #567）と、アカウントの言語の端末への写し。
  // - 明示選択があり保存値と違う場合：選択を勝たせたまま表示し、プロフィール（profiles.locale）にも保存する。
  // - 明示選択が無い場合：従来どおりプロフィール保存値を採用し、端末へ写す。
  useEffect(() => {
    if (!session) {
      // ログアウトで引き継ぎは終わり。明示選択の記録もここで消費する（別アカウントへの持ち越しを防ぐ）。
      // 保存の失敗からの取り直しは、再読み込み・再訪（マウントし直し）では記録が残るので行われる。
      handoffAttempt.current = null;
      clearLocaleExplicitChoice();
      // eslint-disable-next-line react-hooks/set-state-in-effect
      if (handoff !== null) setHandoff(null);
      return;
    }
    if (!account || !token) return;
    const stored = readStoredLocale();
    if (!hasLocaleExplicitChoice()) {
      if (handoff !== null || account === stored) return;
      storeLocale(account);
      notifyLocaleChanged();
      return;
    }
    if (account === stored) {
      clearLocaleExplicitChoice();
      if (handoff !== null) setHandoff(null);
      return;
    }
    const attemptKey = `${token}:${stored}`;
    if (handoffAttempt.current === attemptKey) return;
    handoffAttempt.current = attemptKey;
    setHandoff(stored);
    // 進行中の保存は、表示の切り替え（handoff の変化）による効果の再実行で取り消さない。
    // ログアウト・再ログインで試行の記録が変わっていたら、その保存はもう今の人のものではないので捨てる。
    void (async () => {
      try {
        await apiFetch("/v1/me/locale", {
          method: "PATCH",
          token,
          body: JSON.stringify({ locale: stored }),
        });
      } catch {
        // 保存に失敗しても、選んだ言語の表示はこの場では残す。記録は残るので再読み込みで取り直す。
        return;
      }
      if (handoffAttempt.current !== attemptKey) return;
      clearLocaleExplicitChoice();
      try {
        await refreshProfile();
      } catch {
        // 次にプロフィールを読み直したときに揃う（マイページの保存と同じ扱い）。
      }
      if (handoffAttempt.current !== attemptKey) return;
      setHandoff(null);
    })();
  }, [session, account, token, refreshProfile, handoff]);

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

  // 未ログイン中の選択（プロンプト・登録画面・ログイン画面・登録直後の言語選択）は明示選択として記録する。
  // ログイン時に保存値と違う場合は引き継ぐ（本体 #567）。アカウント側の同期やマイページの保存では立てない。
  const chooseDeviceLocale = useCallback((next: Locale) => {
    storeLocale(next);
    markLocaleExplicitChoice();
    notifyLocaleChanged();
  }, []);

  const previewLocale = useCallback(
    (next: Locale | null) => {
      setPreview(next ? { locale: next, path: pathname } : null);
    },
    [pathname],
  );

  const value = useMemo(
    () => ({ locale: applied, chooseDeviceLocale, previewLocale, preview: activePreview }),
    [applied, chooseDeviceLocale, previewLocale, activePreview],
  );

  return (
    <LocaleContext.Provider value={value}>
      {/* 言語が変わったら中身を描き直す。L はこの描き直しで新しい言語を引く。 */}
      <Fragment key={applied}>{children}</Fragment>
    </LocaleContext.Provider>
  );
}

export function useLocale(): Locale {
  return useContext(LocaleContext).locale;
}

export function useLocaleControls(): LocaleState {
  return useContext(LocaleContext);
}

// サーバーで日本語で作ったタイトルの部分（ja）に、今の表示言語での文言（translated）を対応づける。
export function useTitleTranslation(ja: string, translated: string) {
  useEffect(() => {
    registerTitlePart(ja, translated);
  }, [ja, translated]);
}
