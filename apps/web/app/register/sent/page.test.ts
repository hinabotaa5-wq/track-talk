/** @vitest-environment jsdom */
import { createElement } from "react";
import { act } from "react";
import { createRoot, type Root } from "react-dom/client";
import { afterEach, describe, expect, it, vi } from "vitest";

const resend = vi.hoisted(() => vi.fn());
vi.mock("@/lib/supabase", () => ({
  createClient: () => ({ auth: { resend } }),
}));

import RegisterSentPage from "./page";

async function renderPage() {
  const div = document.createElement("div");
  document.body.appendChild(div);
  let root: Root;
  await act(async () => {
    root = createRoot(div);
    root.render(createElement(RegisterSentPage));
  });
  return {
    div,
    async unmount() {
      await act(async () => {
        root.unmount();
      });
      div.remove();
    },
  };
}

afterEach(() => {
  sessionStorage.clear();
  resend.mockReset();
});

describe("RegisterSentPage", () => {
  it("送ったアドレスと届かないときの案内を出し、同じアドレスに再送できる", async () => {
    sessionStorage.setItem("tracktalk-signup-sent-email", "runner@example.com");
    resend.mockResolvedValue({ error: null });
    const view = await renderPage();
    try {
      expect(view.div.querySelector("h1")?.textContent).toBe("確認メールを送信しました");
      expect(view.div.querySelector('[data-testid="sent-email"]')?.textContent).toBe("runner@example.com");
      expect(view.div.textContent).toContain("迷惑メール");

      const button = Array.from(view.div.querySelectorAll("button")).find((b) => b.textContent === "確認メールを再送する");
      expect(button).toBeDefined();
      await act(async () => {
        button!.click();
      });
      expect(resend).toHaveBeenCalledWith(
        expect.objectContaining({ type: "signup", email: "runner@example.com" }),
      );
      expect(view.div.textContent).toContain("もう一度送信しました");
    } finally {
      await view.unmount();
    }
  });

  it("アドレスが分からないときは、アドレスと再送ボタンを出さない", async () => {
    const view = await renderPage();
    try {
      expect(view.div.querySelector('[data-testid="sent-email"]')).toBeNull();
      expect(view.div.textContent).toContain("入力したメールアドレス");
      expect(Array.from(view.div.querySelectorAll("button")).some((b) => b.textContent === "確認メールを再送する")).toBe(false);
    } finally {
      await view.unmount();
    }
  });
});
