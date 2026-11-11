"use client";

// TEMPORARY（本体 #521 追加要件・運営者決定 2026-11-10）：
// 未ログインでアプリを開いたときの言語選択プロンプト。海外コーチが登録・ログイン画面に
// 到達する前に表示言語を切り替えられるようにするための暫定実装。
// 海外コーチとの DM でのやりとりが終わったら外す（言語設定の仕組み自体は残す）。
// 表示するのは米国（US）からのアクセスのときだけ（本体 #565・運営者決定 2026-11-12）。
// 国コードは Netlify の端（`netlify/edge-functions/geo-country.ts`）が返す。US と確定しないとき
// （US 以外・取れない・失敗）は出さず、従来どおり日本語のまま。切り替えは登録画面とマイページから。
// 外し方：このファイルとテスト、`netlify/edge-functions/geo-country.ts` とそのテストを消し、
// `app/layout.tsx` の `<LocalePrompt />` の1行を消す。

import { useEffect, useId, useRef, useState, useSyncExternalStore } from "react";
import { usePathname } from "next/navigation";
import { Languages } from "lucide-react";
import { useAuth } from "./auth-provider";
import { localePromptAnswered, markLocalePromptAnswered, type Locale } from "@/lib/i18n";
import { useLocaleControls } from "./locale-provider";

function subscribeToNothing() {
  return () => {};
}

// 英語固定の海外コーチ向け紹介ページ（本体 #538）は表示言語設定の対象外なので、DM から開いた人に重ねて出さない。
const exemptPaths = new Set(["/for-coaches"]);

// アクセス元の国コード（大文字2文字）。取れないときや失敗したときは null（＝出さない）。
export async function fetchAccessCountry(): Promise<string | null> {
  try {
    const response = await fetch("/geo-country", { cache: "no-store", credentials: "omit" });
    if (!response.ok) return null;
    const body: unknown = await response.json();
    const code = (body as { country?: unknown } | null)?.country;
    return typeof code === "string" && /^[A-Z]{2}$/.test(code) ? code : null;
  } catch {
    return null;
  }
}

export function LocalePrompt() {
  const pathname = usePathname();
  const { session, sessionLoading } = useAuth();
  const { chooseDeviceLocale } = useLocaleControls();
  // サーバー描画とハイドレーションでは出さず、描画後に端末の記録を読む（食い違いで出そこねないように。本体 #530）。
  const stored = useSyncExternalStore(subscribeToNothing, localePromptAnswered, () => true);
  const [answeredNow, setAnswered] = useState(false);
  const answered = stored || answeredNow;
  const dialog = useRef<HTMLDialogElement>(null);
  const titleId = useId();
  const eligible = !sessionLoading && !session && !answered && !exemptPaths.has(pathname ?? "");
  // undefined はまだ問い合わせていない。出す必要がありそうなときだけ1度問い合わせる。
  const [country, setCountry] = useState<string | null | undefined>(undefined);
  const open = eligible && country === "US";

  useEffect(() => {
    if (!eligible || country !== undefined) return;
    let cancelled = false;
    void fetchAccessCountry().then((code) => {
      if (!cancelled) setCountry(code);
    });
    return () => {
      cancelled = true;
    };
  }, [eligible, country]);

  useEffect(() => {
    const element = dialog.current;
    if (!open || !element || element.open) return;
    element.showModal();
    return () => {
      if (element.open) element.close();
    };
  }, [open ]);

  if (!open) return null;

  // どちらか選ぶまで閉じられない（運営者指示）。Esc でも閉じない。
  function choose(next: Locale) {
    // 回答済みの記録を先に残す（言語の切り替えで画面を描き直しても、もう一度出さない）。
    markLocalePromptAnswered();
    setAnswered(true);
    // 端末に保存して、ヘッダーを含む画面全体へ即時に反映する（登録・ログイン画面の選択欄にも引き継ぐ。本体 #530）。
    chooseDeviceLocale(next);
  }

  return (
    <dialog
      ref={dialog}
      aria-labelledby={titleId}
      onCancel={(event) => {
        event.preventDefault();
      }}
      className="fixed inset-x-0 top-auto bottom-0 m-0 mx-auto max-h-[90dvh] w-full max-w-md overflow-y-auto rounded-t-3xl border border-white/20 bg-slate-900 p-5 pb-[calc(1.25rem+env(safe-area-inset-bottom))] text-slate-100 shadow-2xl backdrop:bg-black/60 sm:inset-y-0 sm:my-auto sm:rounded-3xl sm:pb-5"
    >
      <div className="space-y-3">
        <span className="inline-flex size-10 items-center justify-center rounded-full bg-blue-500/20 text-blue-200">
          <Languages aria-hidden className="size-5" />
        </span>
        <h2 id={titleId} className="text-lg font-bold">表示言語を選んでください / Choose your language</h2>
        <p className="text-sm leading-relaxed text-slate-300">あとから登録時の設定やマイページで変えられます / You can change it later during registration or in My Page settings.</p>
        <div className="grid grid-cols-2 gap-2">
          <button
            type="button"
            onClick={() => choose("ja")}
            className="min-h-11 rounded-md bg-blue-600 px-4 text-sm font-semibold text-pure-white transition active:scale-[0.98]"
          >日本語</button>
          <button
            type="button"
            onClick={() => choose("en")}
            className="min-h-11 rounded-md bg-blue-600 px-4 text-sm font-semibold text-pure-white transition active:scale-[0.98]"
          >
            English
          </button>
        </div>
      </div>
    </dialog>
  );
}
