"use client";

import { parseLocale, t, type I18nKey, type Locale } from "@/lib/i18n";

// 表示言語の選択欄（登録・ログイン・マイページで共通。本体 #530）。
// 選択肢の言語名は、どちらの表示でもその言語自身の表記（日本語／English）にする。
export function LanguageSelect({
  value,
  onChange,
  hint,
  label = "register.language",
}: {
  value: Locale;
  onChange: (next: Locale) => void;
  hint?: I18nKey;
  label?: I18nKey;
}) {
  return (
    <label className="block text-sm font-medium">
      {t(value, label)}
      <select
        className="mt-1 w-full rounded-md border border-white/25 glass bg-white/10 px-3 py-2 text-base text-slate-100 outline-none focus:border-blue-400"
        value={value}
        onChange={(e) => onChange(parseLocale(e.target.value))}
      >
        <option value="ja" className="text-slate-900">{t(value, "register.language.ja")}</option>
        <option value="en" className="text-slate-900">{t(value, "register.language.en")}</option>
      </select>
      {hint ? <span className="mt-1 block text-xs font-normal text-slate-300">{t(value, hint)}</span> : null}
    </label>
  );
}
