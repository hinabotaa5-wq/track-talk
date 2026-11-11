"use client";

import { useState } from "react";
import { useAuth } from "./auth-provider";
import { useLocaleControls } from "./locale-provider";
import { Button } from "./ui/button";
import { apiFetch } from "@/lib/api";
import { parseLocale, storeLocale, notifyLocaleChanged, t, type Locale, L } from "@/lib/i18n";

const choices: readonly { id: Locale; label: string }[] = [
  { id: "ja", label: "日本語" }, // i18n-data（言語名はその言語自身の表記）
  { id: "en", label: "English" },
];

// 「表示言語 / Language」の選択（本体 #530・#536）。マイページの設定一覧から開く /me/language に置く。
// 選ぶとすぐに画面全体が切り替わり（試し表示）、「保存」でアカウント（profiles.locale）に確定する。
// 保存せずに画面を移ると、保存済みの言語に戻る。
// 保存は言語だけを送る PATCH /v1/me/locale（#535）。PATCH /v1/me はニックネーム必須のため使わない。
export function LanguageSwitch({ labelledBy }: { labelledBy?: string }) {
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
    <div className="space-y-2 rounded-2xl border border-white/20 glass bg-white/10 p-4">
      <div role="radiogroup" aria-labelledby={labelledBy} className="grid grid-cols-2 gap-1 rounded-full border border-white/25 bg-white/8 p-1">
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
              className={`flex h-10 min-w-0 items-center justify-center rounded-full px-2 text-sm font-semibold transition-colors active:scale-[0.98] ${
                active ? "bg-blue-600 text-pure-white" : "text-slate-200 hover:text-blue-300"
              }`}
            >
              {label}
            </button>
          );
        })}
      </div>
      <p className="text-xs text-slate-300">{t(locale, "profile.languageHint")}</p>
      {dirty ? (
        <Button type="button" className="w-full" disabled={status === "saving" || !token} onClick={save}>
          {status === "saving" ? t(locale, "common.sending") : t(locale, "common.save")}
        </Button>
      ) : null}
      {status === "error" ? <p role="alert" className="text-sm text-red-400">{L("保存できませんでした。もう一度お試しください。")}</p> : null}
    </div>
  );
}
