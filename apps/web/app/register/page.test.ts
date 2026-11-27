/** @vitest-environment jsdom */
import { createElement } from "react";
import { act } from "react";
import { createRoot, type Root } from "react-dom/client";
import { beforeEach, describe, expect, it, vi } from "vitest";

const router = vi.hoisted(() => ({ push: vi.fn(), replace: vi.fn() }));
const signUp = vi.hoisted(() => vi.fn());
const isPwnedPassword = vi.hoisted(() => vi.fn());
vi.mock("next/navigation", () => ({
  useRouter: () => router,
}));
vi.mock("@/lib/supabase", () => ({
  createClient: () => ({ auth: { signUp } }),
}));
// HIBP の実通信をしない。既定は通過、必要なテストだけ漏洩ありにする。
vi.mock("@/lib/pwned-password", () => ({
  pwnedPasswordMessage: "このパスワードは漏洩情報に含まれているため使えません。別のパスワードを設定してください。",
  isPwnedPassword,
}));

import RegisterPage from "./page";

async function renderPage() {
  const div = document.createElement("div");
  document.body.appendChild(div);
  let root: Root;
  await act(async () => {
    root = createRoot(div);
    root.render(createElement(RegisterPage));
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

function buttons(div: HTMLElement) {
  return Array.from(div.querySelectorAll("button"));
}

describe("RegisterPage", () => {
  beforeEach(() => {
    isPwnedPassword.mockResolvedValue(false);
    signUp.mockReset();
    router.push.mockClear();
    router.replace.mockClear();
  });

  it("表示言語の選択は出さない（運営者決定 2026-11-13）", async () => {
    const view = await renderPage();
    try {
      expect(view.div.querySelector("select")).toBeNull();
      expect(view.div.textContent).not.toContain("表示言語");
    } finally {
      await view.unmount();
    }
  });

  it("同意するまで登録・Google登録を押せない", async () => {
    const view = await renderPage();
    try {
      const checkbox = view.div.querySelector('input[type="checkbox"]');
      expect(checkbox).not.toBeNull();
      const links = Array.from(view.div.querySelectorAll("a")).map((a) => a.getAttribute("href"));
      expect(links).toContain("/terms");
      expect(links).toContain("/guidelines");
      expect(links).toContain("/privacy");

      const before = buttons(view.div);
      const submit = before.find((b) => b.textContent === "登録");
      const google = before.find((b) => b.textContent?.includes("Googleで登録"));
      expect(submit?.disabled).toBe(true);
      expect(google?.disabled).toBe(true);

      await act(async () => {
        (checkbox as HTMLInputElement).click();
      });

      const after = buttons(view.div);
      expect(after.find((b) => b.textContent === "登録")?.disabled).toBe(false);
      expect(after.find((b) => b.textContent?.includes("Googleで登録"))?.disabled).toBe(false);
    } finally {
      await view.unmount();
    }
  });

  it("確認メールを送ったら、同意の版を残したまま送信済みの画面へ移す。アドレスは URL に載せない", async () => {
    signUp.mockResolvedValue({ data: { user: { identities: [{ id: "x" }] }, session: null }, error: null });
    const view = await renderPage();
    try {
      const setValue = (el: HTMLInputElement, value: string) => {
        Object.getOwnPropertyDescriptor(HTMLInputElement.prototype, "value")!.set!.call(el, value);
        el.dispatchEvent(new Event("input", { bubbles: true }));
      };
      await act(async () => {
        setValue(view.div.querySelector('input[type="email"]') as HTMLInputElement, "runner@example.com");
        setValue(view.div.querySelector('input[type="password"]') as HTMLInputElement, "password1234");
        // 前のテストで付けたチェックは画面を離れても残るので、外れているときだけ付ける。
        const checkbox = view.div.querySelector('input[type="checkbox"]') as HTMLInputElement;
        if (!checkbox.checked) checkbox.click();
      });
      await act(async () => {
        view.div.querySelector("form")!.dispatchEvent(new Event("submit", { bubbles: true, cancelable: true }));
      });

      await vi.waitFor(() => expect(router.push).toHaveBeenCalled());
      expect(signUp).toHaveBeenCalledTimes(1);
      expect(signUp.mock.calls[0][0].options.data).toEqual({
        overseas_consent_version: "overseas-2026-10",
        document_consent_versions: {
          terms: "terms-2026-11-26b",
          guidelines: "guidelines-2026-11-25",
          privacy: "privacy-2026-11-26",
        },
      });
      expect(router.push).toHaveBeenCalledWith("/register/sent");
      expect(sessionStorage.getItem("tracktalk-signup-sent-email")).toBe("runner@example.com");
    } finally {
      await view.unmount();
    }
  });

  it("漏洩済みパスワードでは登録せず案内を出す", async () => {
    isPwnedPassword.mockResolvedValue(true);
    const view = await renderPage();
    try {
      const setValue = (el: HTMLInputElement, value: string) => {
        Object.getOwnPropertyDescriptor(HTMLInputElement.prototype, "value")!.set!.call(el, value);
        el.dispatchEvent(new Event("input", { bubbles: true }));
      };
      await act(async () => {
        setValue(view.div.querySelector('input[type="email"]') as HTMLInputElement, "leaked@example.com");
        setValue(view.div.querySelector('input[type="password"]') as HTMLInputElement, "password");
        const checkbox = view.div.querySelector('input[type="checkbox"]') as HTMLInputElement;
        if (!checkbox.checked) checkbox.click();
      });
      await act(async () => {
        view.div.querySelector("form")!.dispatchEvent(new Event("submit", { bubbles: true, cancelable: true }));
      });

      await vi.waitFor(() => expect(view.div.textContent).toContain("漏洩情報"));
      expect(signUp).not.toHaveBeenCalled();
      expect(router.push).not.toHaveBeenCalled();
    } finally {
      await view.unmount();
    }
  });
});

// フィールドエラーの表示（本体 #870）。同意チェック・captcha の条件と送信処理は変えない。
describe("RegisterPage のフィールドエラー表示", () => {
  beforeEach(() => {
    // 上の describe の beforeEach はここには効かない。漏洩テストの true が残らないよう戻す。
    isPwnedPassword.mockResolvedValue(false);
    signUp.mockReset();
  });
  function setValue(div: HTMLElement, type: string, value: string) {
    const el = div.querySelector(`input[type="${type}"]`) as HTMLInputElement;
    Object.getOwnPropertyDescriptor(HTMLInputElement.prototype, "value")!.set!.call(el, value);
    el.dispatchEvent(new Event("input", { bubbles: true }));
  }

  function fieldError(div: HTMLElement, text: string) {
    return Array.from(div.querySelectorAll("p")).find((p) => p.textContent === text);
  }

  it("不正なメール形式で送信すると「メールアドレスを確認してください」が表示される", async () => {
    const view = await renderPage();
    try {
      await act(async () => {
        setValue(view.div, "email", "not-an-email");
        setValue(view.div, "password", "password1234");
        const checkbox = view.div.querySelector('input[type="checkbox"]') as HTMLInputElement;
        if (!checkbox.checked) checkbox.click();
      });
      await act(async () => {
        view.div.querySelector("form")!.dispatchEvent(new Event("submit", { bubbles: true, cancelable: true }));
      });

      await vi.waitFor(() => expect(fieldError(view.div, "メールアドレスを確認してください")).not.toBeUndefined());
      const error = fieldError(view.div, "メールアドレスを確認してください")!;
      expect(error.className).toContain("text-sm");
      expect(error.className).toContain("text-red-400");
      expect(signUp).not.toHaveBeenCalled();
    } finally {
      await view.unmount();
    }
  });

  it("パスワード8文字未満で送信すると「8文字以上にしてください」が表示される", async () => {
    const view = await renderPage();
    try {
      await act(async () => {
        setValue(view.div, "email", "runner@example.com");
        setValue(view.div, "password", "short12");
        const checkbox = view.div.querySelector('input[type="checkbox"]') as HTMLInputElement;
        if (!checkbox.checked) checkbox.click();
      });
      await act(async () => {
        view.div.querySelector("form")!.dispatchEvent(new Event("submit", { bubbles: true, cancelable: true }));
      });

      await vi.waitFor(() => expect(fieldError(view.div, "8文字以上にしてください")).not.toBeUndefined());
      expect(signUp).not.toHaveBeenCalled();
    } finally {
      await view.unmount();
    }
  });

  it("正常な入力ではフィールドエラーが表示されず送信に進む", async () => {
    signUp.mockResolvedValue({ data: { user: { identities: [{ id: "x" }] }, session: null }, error: null });
    const view = await renderPage();
    try {
      await act(async () => {
        setValue(view.div, "email", "runner@example.com");
        setValue(view.div, "password", "password1234");
        const checkbox = view.div.querySelector('input[type="checkbox"]') as HTMLInputElement;
        if (!checkbox.checked) checkbox.click();
      });
      await act(async () => {
        view.div.querySelector("form")!.dispatchEvent(new Event("submit", { bubbles: true, cancelable: true }));
      });

      await vi.waitFor(() => expect(signUp).toHaveBeenCalled());
      expect(view.div.textContent).not.toContain("メールアドレスを確認してください");
      expect(view.div.textContent).not.toContain("パスワードを入力してください");
      expect(view.div.textContent).not.toContain("8文字以上にしてください");
    } finally {
      await view.unmount();
    }
  });
});
