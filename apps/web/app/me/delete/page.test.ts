/** @vitest-environment jsdom */
import { createElement } from "react";
import { act } from "react";
import { createRoot, type Root } from "react-dom/client";
import { beforeEach, describe, expect, it, vi } from "vitest";
import DeleteAccountPage from "./page";

const { state, signInWithPassword, apiFetch, signOut } = vi.hoisted(() => ({
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
    document.cookie = "tracktalk-next=; Path=/; Max-Age=0";
  });

  it("deletes with the current token without asking for a password", async () => {
    apiFetch.mockResolvedValue({ auth_deleted: true });
    const view = await renderPage();
    expect(view.div.querySelector('input[type="password"]')).toBeNull();
    expect(view.div.textContent).not.toContain("再ログイン");
    const phrase = view.div.querySelector('input:not([type="checkbox"])');
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
    await view.unmount();
  });

  async function submit(keep: boolean) {
    apiFetch.mockResolvedValue({ auth_deleted: true });
    const view = await renderPage();
    const phrase = view.div.querySelector('input:not([type="checkbox"])');
    const box = view.div.querySelector('input[type="checkbox"]');
    if (!(phrase instanceof HTMLInputElement) || !(box instanceof HTMLInputElement)) throw new Error("inputs missing");
    await act(async () => {
      setInput(phrase, "退会する");
      if (keep) box.click();
    });
    const form = view.div.querySelector("form");
    if (!form) throw new Error("form missing");
    await act(async () => {
      form.dispatchEvent(new Event("submit", { bubbles: true, cancelable: true }));
    });
    await view.unmount();
    return JSON.parse((apiFetch.mock.calls[0][1] as { body: string }).body);
  }

  it("keeps the injury records only when the box is checked", async () => {
    expect(await submit(false)).toEqual({ confirm: "退会する", keep_injury_anonymized: false });
    apiFetch.mockReset();
    expect(await submit(true)).toEqual({ confirm: "退会する", keep_injury_anonymized: true });
  });
});
