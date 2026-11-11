// @vitest-environment jsdom
import { act, type ReactNode } from "react";
import { createRoot, type Root } from "react-dom/client";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import { LocalePrompt } from "./locale-prompt";

// TEMPORARY（本体 #521 追加要件）のプロンプトのテスト。本体を外すときはこのファイルも消す。

const auth = vi.hoisted(() => ({ session: null as unknown, sessionLoading: false }));
vi.mock("@/components/auth-provider", () => ({
  useAuth: () => ({ session: auth.session, sessionLoading: auth.sessionLoading }),
}));
const nav = vi.hoisted(() => ({ pathname: "/" }));
vi.mock("next/navigation", () => ({ usePathname: () => nav.pathname }));
vi.mock("next/link", () => ({
  default: ({ href, children, ...props }: { href: string; children: ReactNode }) => (
    <a href={href} {...props}>{children}</a>
  ),
}));

let root: Root;
let container: HTMLDivElement;
beforeEach(() => {
  // jsdom は dialog のトップレイヤーを実装していない。
  Object.defineProperties(HTMLDialogElement.prototype, {
    showModal: { configurable: true, value(this: HTMLDialogElement) { this.open = true; } },
    close: { configurable: true, value(this: HTMLDialogElement) { this.open = false; } },
  });
  (globalThis as typeof globalThis & { IS_REACT_ACT_ENVIRONMENT: boolean }).IS_REACT_ACT_ENVIRONMENT = true;
  localStorage.clear();
  auth.session = null;
  auth.sessionLoading = false;
  nav.pathname = "/";
  container = document.createElement("div");
  document.body.appendChild(container);
  root = createRoot(container);
});
afterEach(async () => {
  await act(async () => root.unmount());
  container.remove();
  localStorage.clear();
  Reflect.deleteProperty(HTMLDialogElement.prototype, "showModal");
  Reflect.deleteProperty(HTMLDialogElement.prototype, "close");
});

describe("未ログイン時の言語選択プロンプト", () => {
  it("未ログインの初回は出て、英語を選ぶと保存して閉じる", async () => {
    await act(async () => root.render(<LocalePrompt />));
    expect(container.textContent).toContain("Choose your language");
    await act(async () => {
      container.querySelectorAll("button")[1].dispatchEvent(new MouseEvent("click", { bubbles: true }));
    });
    expect(localStorage.getItem("tracktalk-locale")).toBe("en");
    expect(localStorage.getItem("tracktalk:locale-prompted")).toBe("1");
    expect(container.textContent).not.toContain("Choose your language");
  });

  it("英語固定の海外コーチ向け紹介ページ（/for-coaches）では出さない", async () => {
    nav.pathname = "/for-coaches";
    await act(async () => root.render(<LocalePrompt />));
    expect(container.textContent).not.toContain("Choose your language");
    expect(localStorage.getItem("tracktalk:locale-prompted")).toBeNull();
  });

  it("答えたあとは出ない", async () => {
    localStorage.setItem("tracktalk:locale-prompted", "1");
    await act(async () => root.render(<LocalePrompt />));
    expect(container.textContent).not.toContain("Choose your language");
  });

  it("ログイン中は出ない", async () => {
    auth.session = {};
    await act(async () => root.render(<LocalePrompt />));
    expect(container.textContent).not.toContain("Choose your language");
  });

  it("選ばないと閉じられない（閉じる導線なし・Esc でも閉じない）", async () => {
    await act(async () => root.render(<LocalePrompt />));
    // 閉じるためのボタンはない。
    expect(container.textContent).not.toContain("閉じる");
    // Esc（cancel イベント）でも閉じず、回答記録も付かない。
    await act(async () => {
      container.querySelector("dialog")?.dispatchEvent(new Event("cancel", { bubbles: true, cancelable: true }));
    });
    expect(container.textContent).toContain("Choose your language");
    expect(localStorage.getItem("tracktalk:locale-prompted")).toBeNull();
  });
});
