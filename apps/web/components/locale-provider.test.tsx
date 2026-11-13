// @vitest-environment jsdom
import { act } from "react";
import { createRoot, type Root } from "react-dom/client";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import { L, loadEnglish, setActiveLocale } from "@/lib/i18n";
import { LocaleProvider, useLocale } from "./locale-provider";

// 表示言語の決め方（運営者決定 2026-11-13）：ブラウザ言語の自動判定だけ。
// 端末保存・プロフィール locale・過去の手動選択は使わない（残っていても無視する）。

vi.mock("next/navigation", () => ({ usePathname: () => "/" }));

function Probe() {
  const locale = useLocale();
  return <p data-testid="probe">{`${locale}:${L("フィード")}`}</p>;
}

function setBrowserLanguage(language: unknown, languages: readonly string[] = []) {
  Object.defineProperty(window.navigator, "language", { value: language, configurable: true });
  Object.defineProperty(window.navigator, "languages", { value: languages, configurable: true });
}

async function switchBrowserLanguage(language: string) {
  await act(async () => {
    setBrowserLanguage(language);
    window.dispatchEvent(new Event("languagechange"));
    await loadEnglish();
    await new Promise((resolve) => setTimeout(resolve, 0));
  });
}

let root: Root;
let container: HTMLDivElement;
const realLanguage = Object.getOwnPropertyDescriptor(window.navigator, "language");
const realLanguages = Object.getOwnPropertyDescriptor(window.navigator, "languages");
const probe = () => container.querySelector("[data-testid=probe]")?.textContent;

async function render(children: React.ReactNode) {
  await act(async () => root.render(<LocaleProvider>{children}</LocaleProvider>));
  // 英語の辞書は後から読み込むので、読み終わるまで待つ。
  await act(async () => {
    await loadEnglish();
    await new Promise((resolve) => setTimeout(resolve, 0));
  });
}

beforeEach(() => {
  (globalThis as typeof globalThis & { IS_REACT_ACT_ENVIRONMENT: boolean }).IS_REACT_ACT_ENVIRONMENT = true;
  localStorage.clear();
  setBrowserLanguage("ja-JP");
  container = document.createElement("div");
  document.body.appendChild(container);
  root = createRoot(container);
});
afterEach(async () => {
  await act(async () => root.unmount());
  container.remove();
  localStorage.clear();
  if (realLanguage) Object.defineProperty(window.navigator, "language", realLanguage);
  if (realLanguages) Object.defineProperty(window.navigator, "languages", realLanguages);
  setActiveLocale("ja");
  document.documentElement.lang = "";
});

describe("LocaleProvider（ブラウザ言語の自動判定）", () => {
  it("日本語ブラウザは日本語で出す", async () => {
    await render(<Probe />);
    expect(probe()).toBe("ja:フィード");
    expect(document.documentElement.lang).toBe("ja");
  });

  it("en-US ブラウザはプロンプトなしで最初から英語で出す", async () => {
    const fetchMock = vi.fn();
    vi.stubGlobal("fetch", fetchMock);
    try {
      setBrowserLanguage("en-US");
      await render(<Probe />);
      expect(probe()).toBe("en:Feed");
      expect(document.documentElement.lang).toBe("en");
      expect(fetchMock).not.toHaveBeenCalledWith("/geo-country", expect.anything());
    } finally {
      vi.unstubAllGlobals();
    }
  });

  it("en-GB など en で始まる値も英語、その他（fr-FR 等）は日本語", async () => {
    setBrowserLanguage("en-GB");
    await render(<Probe />);
    expect(probe()).toBe("en:Feed");

    await switchBrowserLanguage("fr-FR");
    await render(<Probe />);
    expect(probe()).toBe("ja:フィード");
  });

  it("端末に旧来の保存があっても無視する", async () => {
    localStorage.setItem("tracktalk-locale", "en");
    localStorage.setItem("tracktalk:locale-prompted", "1");
    localStorage.setItem("tracktalk:locale-explicit", "1");
    await render(<Probe />);
    expect(probe()).toBe("ja:フィード");
  });

  it("ブラウザ言語を変えれば次回表示から追従する", async () => {
    await render(<Probe />);
    expect(probe()).toBe("ja:フィード");
    await switchBrowserLanguage("en-US");
    await render(<Probe />);
    expect(probe()).toBe("en:Feed");
  });
});
