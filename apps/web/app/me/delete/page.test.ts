/** @vitest-environment jsdom */
import { createElement } from "react";
import { act } from "react";
import { createRoot, type Root } from "react-dom/client";
import { beforeEach, describe, expect, it, vi } from "vitest";
import DeleteAccountPage from "./page";

const { state, signInWithPassword, signInWithOAuth, apiFetch, signOut } = vi.hoisted(() => ({
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
  signInWithOAuth: vi.fn(),
  apiFetch: vi.fn(),
  signOut: vi.fn(),
}));

vi.mock("next/navigation", () => ({
  useRouter: () => ({ replace: vi.fn(), refresh: vi.fn() }),
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
    auth: { signInWithPassword, signInWithOAuth, signOut },
  }),
}));

function jwt(claims: object) {
  const payload = btoa(JSON.stringify(claims)).replace(/\+/g, "-").replace(/\//g, "_").replace(/=+$/, "");
  return `eyJhbGciOiJIUzI1NiJ9.${payload}.sig`;
}

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
    signInWithOAuth.mockReset();
    apiFetch.mockReset();
    signOut.mockReset();
    document.cookie = "tracktalk-next=; Path=/; Max-Age=0";
  });

  it("reauthenticates with the password and deletes with the new token", async () => {
    signInWithPassword.mockResolvedValue({
      data: { session: { access_token: "fresh-token" } },
      error: null,
    });
    apiFetch.mockResolvedValue({ auth_deleted: true });
    const view = await renderPage();
    const password = view.div.querySelector('input[type="password"]');
    const phrase = view.div.querySelector('input:not([type="password"])');
    if (!(password instanceof HTMLInputElement) || !(phrase instanceof HTMLInputElement)) {
      throw new Error("inputs missing");
    }
    await act(async () => {
      setInput(phrase, "退会する");
      setInput(password, "secret-pass");
    });
    const form = view.div.querySelector("form");
    if (!form) throw new Error("form missing");
    await act(async () => {
      form.dispatchEvent(new Event("submit", { bubbles: true, cancelable: true }));
    });
    expect(signInWithPassword).toHaveBeenCalledWith({
      email: "runner@example.com",
      password: "secret-pass",
    });
    expect(apiFetch).toHaveBeenCalledWith("/v1/me", expect.objectContaining({ token: "fresh-token" }));
    expect(apiFetch.mock.calls[0][1].token).not.toBe("old-token");
    await view.unmount();
  });

  it("does not delete when the password is wrong", async () => {
    signInWithPassword.mockResolvedValue({
      data: { session: null },
      error: { code: "invalid_credentials", message: "Invalid login credentials" },
    });
    const view = await renderPage();
    const password = view.div.querySelector('input[type="password"]');
    const phrase = view.div.querySelector('input:not([type="password"])');
    if (!(password instanceof HTMLInputElement) || !(phrase instanceof HTMLInputElement)) {
      throw new Error("inputs missing");
    }
    await act(async () => {
      setInput(phrase, "退会する");
      setInput(password, "wrong");
    });
    const form = view.div.querySelector("form");
    if (!form) throw new Error("form missing");
    await act(async () => {
      form.dispatchEvent(new Event("submit", { bubbles: true, cancelable: true }));
    });
    expect(view.div.textContent).toContain("メールアドレスまたはパスワードが違います。");
    expect(apiFetch).not.toHaveBeenCalled();
    await view.unmount();
  });

  it("asks a Google user to sign in again before an old token can withdraw", async () => {
    state.session = {
      user: { email: "runner@example.com", app_metadata: { provider: "google" } },
    };
    state.token = jwt({ auth_time: 1 });
    signInWithOAuth.mockResolvedValue({ error: null });
    const view = await renderPage();
    expect(view.div.querySelector('input[type="password"]')).toBeNull();
    expect(view.div.textContent).toContain("Googleで再ログイン");
    const button = [...view.div.querySelectorAll("button")].find((item) => item.textContent?.includes("Googleで再ログイン"));
    if (!button) throw new Error("google button missing");
    await act(async () => {
      button.dispatchEvent(new MouseEvent("click", { bubbles: true }));
    });
    expect(document.cookie).toContain("tracktalk-next=%2Fme%2Fdelete");
    const form = view.div.querySelector("form");
    if (!form) throw new Error("form missing");
    const phrase = view.div.querySelector("input");
    if (!(phrase instanceof HTMLInputElement)) throw new Error("phrase missing");
    await act(async () => {
      setInput(phrase, "退会する");
      form.dispatchEvent(new Event("submit", { bubbles: true, cancelable: true }));
    });
    expect(apiFetch).not.toHaveBeenCalled();
    await view.unmount();
  });

  it("deletes with a Google session that just signed in", async () => {
    const token = jwt({ auth_time: Math.floor(Date.now() / 1000) });
    state.session = {
      user: { email: "runner@example.com", app_metadata: { provider: "google" } },
    };
    state.token = token;
    apiFetch.mockResolvedValue({ auth_deleted: true });
    const view = await renderPage();
    expect(view.div.textContent).not.toContain("Googleで再ログイン");
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
    expect(apiFetch).toHaveBeenCalledWith("/v1/me", expect.objectContaining({ token }));
    await view.unmount();
  });
});
