// @vitest-environment jsdom
import { afterEach, describe, expect, it } from "vitest";
import { applyThemePreference, parseThemePreference, readThemePreference, saveThemePreference, themeColors, themeStartupScript, themeStorageKey } from "./theme";

function addMeta(media: string, content: string) {
  const meta = document.createElement("meta");
  meta.name = "theme-color";
  meta.setAttribute("media", media);
  meta.content = content;
  document.head.appendChild(meta);
  return meta;
}

afterEach(() => {
  localStorage.clear();
  document.documentElement.removeAttribute("data-theme");
  document.head.innerHTML = "";
});

describe("画面の表示の切り替え", () => {
  it("知らない値は端末の設定に合わせる", () => {
    expect(parseThemePreference("dark")).toBe("dark");
    expect(parseThemePreference("light")).toBe("light");
    expect(parseThemePreference("blue")).toBe("system");
    expect(parseThemePreference(null)).toBe("system");
  });

  it("選んだ表示を保存し、html とステータスバーの色に当てる", () => {
    const light = addMeta("(prefers-color-scheme: light)", themeColors.light);
    const dark = addMeta("(prefers-color-scheme: dark)", themeColors.dark);

    saveThemePreference("dark");
    expect(localStorage.getItem(themeStorageKey)).toBe("dark");
    expect(document.documentElement.dataset.theme).toBe("dark");
    expect(light.content).toBe(themeColors.dark);
    expect(dark.content).toBe(themeColors.dark);
    expect(readThemePreference()).toBe("dark");

    saveThemePreference("system");
    expect(localStorage.getItem(themeStorageKey)).toBeNull();
    expect(document.documentElement.hasAttribute("data-theme")).toBe(false);
    expect(light.content).toBe(themeColors.light);
    expect(dark.content).toBe(themeColors.dark);
  });

  it("起動用スクリプトは保存した表示を描く前に当てる", () => {
    localStorage.setItem(themeStorageKey, "light");
    new Function(themeStartupScript)();
    expect(document.documentElement.dataset.theme).toBe("light");
  });

  it("保存が無ければ起動用スクリプトは何もしない", () => {
    new Function(themeStartupScript)();
    applyThemePreference(readThemePreference());
    expect(document.documentElement.hasAttribute("data-theme")).toBe(false);
  });
});
