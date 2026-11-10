"use client";
import { L } from "@/lib/i18n";

import { useSyncExternalStore } from "react";
import { Monitor, Moon, Sun, type LucideIcon } from "lucide-react";
import { readThemePreference, saveThemePreference, type ThemePreference } from "@/lib/theme";

const options: readonly { id: ThemePreference; label: string; icon: LucideIcon }[] = [
  { id: "light", get label() { return L("ライト"); }, icon: Sun },
  { id: "dark", get label() { return L("ダーク"); }, icon: Moon },
  { id: "system", get label() { return L("端末の設定"); }, icon: Monitor },
];

const listeners = new Set<() => void>();
function subscribeTheme(listener: () => void) {
  listeners.add(listener);
  window.addEventListener("storage", listener);
  return () => {
    listeners.delete(listener);
    window.removeEventListener("storage", listener);
  };
}
function notifyTheme() {
  listeners.forEach((listener) => listener());
}

// マイページの「画面の表示」。この端末にだけ保存する(アカウントには保存しない)。
export function ThemeSwitch() {
  // 保存した値はブラウザでしか読めないので、サーバーで描くときはどれも選ばない。
  const current = useSyncExternalStore(subscribeTheme, readThemePreference, () => null);

  return (
    <section aria-labelledby="theme-switch-title" className="space-y-2 rounded-2xl border border-white/20 glass bg-white/10 p-4">
      <h2 id="theme-switch-title" className="text-sm font-semibold text-slate-100">{L("画面の表示")}</h2>
      <div role="radiogroup" aria-labelledby="theme-switch-title" className="grid grid-cols-3 gap-1 rounded-full border border-white/25 bg-white/8 p-1">
        {options.map(({ id, label, icon: Icon }) => {
          const active = current === id;
          return (
            <button
              key={id}
              type="button"
              role="radio"
              aria-checked={active}
              onClick={() => {
                saveThemePreference(id);
                notifyTheme();
              }}
              className={`flex h-10 min-w-0 items-center justify-center gap-1.5 rounded-full px-2 text-xs font-semibold transition-colors active:scale-[0.98] sm:text-sm ${
                active ? "bg-blue-600 text-pure-white" : "text-slate-200 hover:text-blue-300"
              }`}
            >
              <Icon className="size-4 shrink-0" aria-hidden />
              <span className="truncate">{label}</span>
            </button>
          );
        })}
      </div>
    </section>
  );
}
