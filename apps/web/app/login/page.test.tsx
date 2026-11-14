/** @vitest-environment jsdom */
import { act } from "react";
import { createRoot, type Root } from "react-dom/client";
import { describe, expect, it, vi } from "vitest";

vi.mock("@/components/auth-provider", () => ({
  useAuth: () => ({ session: null, sessionLoading: false }),
}));
// ログイン画面の描画では Supabase を呼ばない。実通信をしない。
vi.mock("@/lib/supabase", () => ({
  createClient: () => null,
}));

import LoginPage from "./page";

(globalThis as { IS_REACT_ACT_ENVIRONMENT?: boolean }).IS_REACT_ACT_ENVIRONMENT = true;

async function renderPage() {
  const div = document.createElement("div");
  document.body.appendChild(div);
  let root: Root | undefined;
  await act(async () => {
    root = createRoot(div);
    root.render(<LoginPage />);
  });
  return {
    div,
    async unmount() {
      await act(async () => {
        root!.unmount();
      });
      div.remove();
    },
  };
}

function links(div: HTMLElement) {
  return Array.from(div.querySelectorAll("a"));
}

describe("LoginPage の下部3導線", () => {
  it("「新規登録」のボタン形リンクがあり href が /register", async () => {
    const view = await renderPage();
    try {
      const register = links(view.div).find((a) => a.textContent === "新規登録");
      expect(register).not.toBeUndefined();
      expect(register!.getAttribute("href")).toBe("/register");
      // Google ボタンと同系統の secondary 相当（全幅・枠線・太字）で、primary の青一色ではない。
      expect(register!.className).toContain("w-full");
      expect(register!.className).toContain("border");
      expect(register!.className).not.toContain("bg-blue-600");
    } finally {
      await view.unmount();
    }
  });

  it("「アカウントがない場合は」はリンク外の独立した見出し行で、文中の「登録」リンクは残っていない", async () => {
    const view = await renderPage();
    try {
      expect(view.div.textContent).toContain("アカウントがない場合は");
      const wordLink = links(view.div).find((a) => a.textContent === "登録");
      expect(wordLink).toBeUndefined();
    } finally {
      await view.unmount();
    }
  });

  it("「パスワードを忘れた」のリンクがあり href が /reset-password", async () => {
    const view = await renderPage();
    try {
      const forgot = links(view.div).find((a) => a.textContent === "パスワードを忘れた");
      expect(forgot).not.toBeUndefined();
      expect(forgot!.getAttribute("href")).toBe("/reset-password");
    } finally {
      await view.unmount();
    }
  });

  it("「ログインできないときのお問い合わせ」のリンクがあり href が /contact", async () => {
    const view = await renderPage();
    try {
      const contact = links(view.div).find((a) => a.textContent === "ログインできないときのお問い合わせ");
      expect(contact).not.toBeUndefined();
      expect(contact!.getAttribute("href")).toBe("/contact");
    } finally {
      await view.unmount();
    }
  });

  it("2つのテキストリンクは同じスタイルで行全体がタップ領域", async () => {
    const view = await renderPage();
    try {
      const forgot = links(view.div).find((a) => a.textContent === "パスワードを忘れた")!;
      const contact = links(view.div).find((a) => a.textContent === "ログインできないときのお問い合わせ")!;
      expect(forgot).not.toBeUndefined();
      expect(contact).not.toBeUndefined();
      // 色・サイズ・太さ・下線・hover／active を2つで完全に揃える。
      expect(forgot.className).toBe(contact.className);
      // ボタン形にはしない。
      expect(forgot.className).not.toContain("border");
      // 行全体＋上下 padding で実効タップ高さをおおむね 40px 以上。
      expect(forgot.className).toContain("w-full");
      expect(forgot.className).toContain("min-h-[44px]");
    } finally {
      await view.unmount();
    }
  });

  it("従来どおりメール／パスワード欄と「ログイン」「Googleでログイン」が出る", async () => {
    const view = await renderPage();
    try {
      expect(view.div.querySelector('input[type="email"]')).not.toBeNull();
      expect(view.div.querySelector('input[type="password"]')).not.toBeNull();
      const buttons = Array.from(view.div.querySelectorAll("button")).map((b) => b.textContent);
      expect(buttons).toContain("ログイン");
      expect(buttons.some((t) => t?.includes("Googleでログイン"))).toBe(true);
    } finally {
      await view.unmount();
    }
  });
});
