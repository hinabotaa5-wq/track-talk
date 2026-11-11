"use client";

import { useState } from "react";
import { Languages } from "lucide-react";
import { useAuth } from "./auth-provider";
import { useLocaleControls } from "./locale-provider";
import { Button } from "./ui/button";
import { apiFetch } from "@/lib/api";
import { parseLocale, storeLocale, notifyLocaleChanged, t, type Locale, L } from "@/lib/i18n";

const choices: readonly { id: Locale; label: string }[] = [
  { id: "ja", label: "日本語" }, // i18n-data（言語名はその言語自身の表記）
  { id: "en", label: "English" },
];

// マイページの設定一覧にある「表示言語 / Language」の1行（本体 #530・#536）。
// 選ぶとすぐに画面全体が切り替わり（試し表示）、「保存」でアカウント（profiles.locale）に確定する。
// 保存せずに画面を移ると、保存済みの言語に戻る。
// 保存は言語だけを送る PATCH /v1/me/locale（#535）。PATCH /v1/me はニックネーム必須のため使わない。
export function LanguageSwitch() {
  const { token, profile, refreshProfile } = useAuth();
  const { locale, preview, previewLocale } = useLocaleControls();
  const saved = parseLocale(profile?.locale);
  const [status, setStatus] = useState<"idle" | "saving" | "error">("idle");
  const dirty = preview !== null && preview !== saved;

  async function save() {
    if (!token || preview === null) return;
    setStatus("saving");
    try {
      await apiFetch("/v1/me/locale", { method: "PATCH", token, body: JSON.stringify({ locale: preview }) });
    } catch {
      setStatus("error");
      return;
    }
    // 保存はできている。プロフィールの取り直しに失敗しても、保存の失敗としては出さない。
    storeLocale(preview);
    notifyLocaleChanged();
    try {
      await refreshProfile();
    } catch {
      // 次にプロフィールを読み直したときに揃う。
    }
    previewLocale(null);
    setStatus("idle");
  }

  return (
    <div className="space-y-2 px-4 py-3">
      <div className="flex items-center gap-3">
        <Languages className="size-4 shrink-0 text-blue-300" aria-hidden />
        <span id="language-switch-title" className="flex-1 text-sm font-medium text-slate-100">
          {t(locale, "profile.language")}
        </span>
        <div role="radiogroup" aria-labelledby="language-switch-title" className="flex shrink-0 gap-1 rounded-full border border-white/25 bg-white/8 p-0.5">
          {choices.map(({ id, label }) => {
            const active = locale === id;
            return (
              <button
                key={id}
                type="button"
                role="radio"
                aria-checked={active}
                lang={id}
                onClick={() => {
                  setStatus("idle");
                  previewLocale(id === saved ? null : id);
                }}
                className={`flex h-8 min-w-16 items-center justify-center rounded-full px-3 text-xs font-semibold transition-colors active:scale-[0.98] ${
                  active ? "bg-blue-600 text-pure-white" : "text-slate-200 hover:text-blue-300"
                }`}
              >
                {label}
              </button>
            );
          })}
        </div>
      </div>
      {dirty ? (
        <div className="space-y-2 pl-7">
          <p className="text-xs text-slate-300">{t(locale, "profile.languageHint")}</p>
          <Button type="button" className="w-full" disabled={status === "saving" || !token} onClick={save}>
            {status === "saving" ? t(locale, "common.sending") : t(locale, "common.save")}
          </Button>
        </div>
      ) : null}
      {status === "error" ? <p role="alert" className="pl-7 text-sm text-red-400">{L("保存できませんでした。もう一度お試しください。")}</p> : null}
    </div>
  );
}
