/** @vitest-environment jsdom */
import { createElement } from "react";
import { act } from "react";
import { createRoot, type Root } from "react-dom/client";
import { beforeEach, describe, expect, it, vi } from "vitest";
import DeleteAccountPage from "./page";

const { state, signInWithPassword, apiFetch, signOut, replace } = vi.hoisted(() => ({
  state: {
    session: {
      user: { email: "runner@example.com", app_metadata: { provider: "email" } },
    } as {
      user: { email?: string; app_metadata?: { provider?: string } };
    } | null,
    token: "old-token" as string | null,
    loading: false,
  },
  signInWithPassword: vi.fn(),
  apiFetch: vi.fn(),
  signOut: vi.fn(),
  replace: vi.fn(),
}));

vi.mock("next/navigation", () => ({
  useRouter: () => ({ replace, refresh: vi.fn() }),
}));

vi.mock("@/components/auth-provider", () => ({
  useAuth: () => state,
}));

vi.mock("@/lib/api", () => ({
  apiFetch: (...args: unknown[]) => apiFetch(...args),
  ApiRequestError: class ApiRequestError extends Error {
    fields?: Record<string, string>;
  },
}));

vi.mock("@/lib/supabase", () => ({
  createClient: () => ({
    auth: { signInWithPassword, signOut },
  }),
}));

function setInput(input: HTMLInputElement, value: string) {
  const setter = Object.getOwnPropertyDescriptor(HTMLInputElement.prototype, "value")?.set;
  setter?.call(input, value);
  input.dispatchEvent(new Event("input", { bubbles: true }));
}

async function renderPage() {
  const div = document.createElement("div");
  document.body.appendChild(div);
  let root: Root;
  await act(async () => {
    root = createRoot(div);
    root.render(createElement(DeleteAccountPage));
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

describe("DeleteAccountPage", () => {
  beforeEach(() => {
    state.session = {
      user: { email: "runner@example.com", app_metadata: { provider: "email" } },
    };
    state.token = "old-token";
    state.loading = false;
    signInWithPassword.mockReset();
    apiFetch.mockReset();
    signOut.mockReset();
    replace.mockReset();
    document.cookie = "tracktalk-next=; Path=/; Max-Age=0";
  });

  it("deletes with the current token without asking for a password", async () => {
    apiFetch.mockResolvedValue({ auth_deleted: true });
    const view = await renderPage();
    expect(view.div.querySelector('input[type="password"]')).toBeNull();
    expect(view.div.textContent).not.toContain("再ログイン");
    const phrase = view.div.querySelector("input");
    if (!(phrase instanceof HTMLInputElement)) throw new Error("phrase missing");
    await act(async () => {
      setInput(phrase, "退会する");
    });
    const form = view.div.querySelector("form");
    if (!form) throw new Error("form missing");
    await act(async () => {
      form.dispatchEvent(new Event("submit", { bubbles: true, cancelable: true }));
    });
    expect(signInWithPassword).not.toHaveBeenCalled();
    expect(apiFetch).toHaveBeenCalledWith("/v1/me", expect.objectContaining({ token: "old-token" }));
    expect(signOut).toHaveBeenCalled();
    expect(replace).toHaveBeenCalledWith("/goodbye");
    await view.unmount();
  });

  // 本体 #468：淡い水色背景の案内ボックスは本文と同じ濃い文字色にする。
  it("上部の案内ボックスは白文字ではなく本文色で読める", async () => {
    apiFetch.mockImplementation((path: string) =>
      Promise.resolve(path === "/v1/coach/me" ? { application: { status: "approved" } } : {}),
    );
    const view = await renderPage();
    await act(async () => {});
    const box = Array.from(view.div.querySelectorAll("p")).find((p) =>
      p.textContent?.includes("コーチだけをやめたいときは"),
    );
    if (!(box instanceof HTMLParagraphElement)) throw new Error("案内ボックスが表示されていない");
    expect(box.className).toContain("text-slate-300");
    expect(box.className).not.toContain("text-pure-white");
    expect(box.className).not.toContain("text-white");
    await view.unmount();
  });

  // リンク「コーチをやめる」は青文字＋下線のまま本文と見分けられる。文面は変えない。
  it("案内ボックス内のリンクは青文字＋下線で文面は現状のまま", async () => {
    apiFetch.mockImplementation((path: string) =>
      Promise.resolve(path === "/v1/coach/me" ? { application: { status: "approved" } } : {}),
    );
    const view = await renderPage();
    await act(async () => {});
    const box = Array.from(view.div.querySelectorAll("p")).find((p) =>
      p.textContent?.includes("コーチだけをやめたいときは"),
    );
    if (!(box instanceof HTMLParagraphElement)) throw new Error("案内ボックスが表示されていない");
    const link = box.querySelector("a");
    expect(link?.getAttribute("href")).toBe("/coach/withdraw");
    expect(link?.className).toContain("text-blue-300");
    expect(link?.className).toContain("underline");
    expect(box.textContent).toContain("送金が済んでから退会してください。");
    await view.unmount();
  });
});
