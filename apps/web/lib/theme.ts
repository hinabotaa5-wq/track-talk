// 画面の表示(ライト・ダーク・端末の設定に合わせる)。マイページで選び、この端末にだけ保存する。
// 選んだ表示は html の data-theme に入れ、globals.css がその値で色を切り替える。
// 「端末の設定に合わせる」のときは data-theme を外し、prefers-color-scheme に任せる。

export type ThemePreference = "system" | "light" | "dark";

export const themeStorageKey = "tracktalk-theme";

// ヘッダーと同じ色。layout.tsx の themeColor と globals.css の --background に合わせる。
export const themeColors = { light: "#f2f2f7", dark: "#17171a" } as const;

export function parseThemePreference(value: unknown): ThemePreference {
  return value === "light" || value === "dark" ? value : "system";
}

export function readThemePreference(): ThemePreference {
  try {
    return parseThemePreference(window.localStorage.getItem(themeStorageKey));
  } catch {
    return "system";
  }
}

// 起動用スクリプトと同じ処理。ステータスバーの色(theme-color)も選んだ表示にそろえる。
export function applyThemePreference(preference: ThemePreference) {
  const root = document.documentElement;
  if (preference === "system") root.removeAttribute("data-theme");
  else root.setAttribute("data-theme", preference);
  document.querySelectorAll<HTMLMetaElement>('meta[name="theme-color"]').forEach((meta) => {
    const media = meta.getAttribute("media") ?? "";
    const own = media.includes("dark") ? themeColors.dark : themeColors.light;
    meta.content = preference === "system" ? own : themeColors[preference];
  });
}

export function saveThemePreference(preference: ThemePreference) {
  try {
    if (preference === "system") window.localStorage.removeItem(themeStorageKey);
    else window.localStorage.setItem(themeStorageKey, preference);
  } catch {
    // 保存できない(プライベートブラウズなど)ときも、開いているあいだは切り替える。
  }
  applyThemePreference(preference);
}

// 画面を描く前に読む。React を待つと、一瞬だけ別の表示が見えてしまう。
// theme-color の meta はまだ無いことがあるので、DOM が揃ったあとにもう一度合わせる。
export const themeStartupScript = `(()=>{try{const t=localStorage.getItem(${JSON.stringify(themeStorageKey)});if(t!=="light"&&t!=="dark")return;document.documentElement.setAttribute("data-theme",t);const c=t==="dark"?${JSON.stringify(themeColors.dark)}:${JSON.stringify(themeColors.light)};const f=()=>document.querySelectorAll('meta[name="theme-color"]').forEach((m)=>{m.content=c;});f();document.addEventListener("DOMContentLoaded",f);}catch{}})();`;
