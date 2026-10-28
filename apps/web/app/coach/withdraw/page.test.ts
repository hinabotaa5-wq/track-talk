/** @vitest-environment jsdom */
import { createElement } from "react";
import { act } from "react";
import { createRoot, type Root } from "react-dom/client";
import { beforeEach, describe, expect, it, vi } from "vitest";
import CoachWithdrawPage from "./page";

const { state, apiFetch, signOut, replace } = vi.hoisted(() => ({
  state: { session: { user: {} } as object | null, token: "token" as string | null, loading: false },
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
  createClient: () => ({ auth: { signOut } }),
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
    root.render(createElement(CoachWithdrawPage));
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

describe("CoachWithdrawPage", () => {
  beforeEach(() => {
    apiFetch.mockReset();
    signOut.mockReset();
    replace.mockReset();
  });

  it("stops coaching without deleting or signing out the account", async () => {
    apiFetch.mockImplementation((path: string) =>
      Promise.resolve({ application: { status: path === "/v1/coach/me" ? "approved" : "withdrawn" } }),
    );
    const view = await renderPage();
    const phrase = view.div.querySelector("input");
    if (!(phrase instanceof HTMLInputElement)) throw new Error("phrase missing");
    await act(async () => {
      setInput(phrase, "コーチをやめる");
    });
    const form = view.div.querySelector("form");
    if (!form) throw new Error("form missing");
    await act(async () => {
      form.dispatchEvent(new Event("submit", { bubbles: true, cancelable: true }));
    });
    expect(apiFetch).toHaveBeenCalledWith("/v1/coach/me/withdraw", expect.objectContaining({ method: "POST" }));
    expect(apiFetch).not.toHaveBeenCalledWith("/v1/me", expect.objectContaining({ method: "DELETE" }));
    expect(signOut).not.toHaveBeenCalled();
    expect(view.div.textContent).toContain("コーチをやめました");
    await view.unmount();
  });

  it("does not offer the button to people who are not coaching", async () => {
    apiFetch.mockResolvedValue({ application: null });
    const view = await renderPage();
    expect(view.div.querySelector("form")).toBeNull();
    expect(view.div.textContent).toContain("いまはコーチとして活動していません");
    await view.unmount();
  });
});
