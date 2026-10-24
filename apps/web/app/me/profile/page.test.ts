/** @vitest-environment jsdom */
import { createElement } from "react";
import { act } from "react";
import { createRoot, type Root } from "react-dom/client";
import { beforeEach, describe, expect, it, vi } from "vitest";
import ProfilePage from "./page";

const analytics = vi.hoisted(() => ({ track: vi.fn(), save: vi.fn() }));
vi.mock("@/lib/analytics", () => ({ trackEvent: analytics.track }));
vi.mock("@/lib/api", async (original) => ({ ...await original<typeof import("@/lib/api")>(), apiFetch: analytics.save }));

const { state } = vi.hoisted(() => ({
  state: {
    session: { user: { id: "user" } } as { user: { id: string } } | null,
    token: "token" as string | null,
    profile: { nickname: "ランナー", nickname_public: false } as {
      nickname: string | null;
      nickname_public: boolean;
      event?: string;
    } | null,
    loading: false,
  },
}));

vi.mock("next/navigation", () => ({
  useRouter: () => ({ replace: vi.fn() }),
}));

vi.mock("@/components/auth-provider", () => ({
  useAuth: () => ({ ...state, refreshProfile: vi.fn() }),
}));

function setInput(input: HTMLInputElement, value: string) {
  const setter = Object.getOwnPropertyDescriptor(HTMLInputElement.prototype, "value")?.set;
  setter?.call(input, value);
  input.dispatchEvent(new Event("input", { bubbles: true }));
  input.dispatchEvent(new Event("change", { bubbles: true }));
}

async function renderPage() {
  const div = document.createElement("div");
  document.body.appendChild(div);
  let root: Root;
  await act(async () => {
    root = createRoot(div);
    root.render(createElement(ProfilePage));
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

describe("ProfilePage", () => {
  beforeEach(() => {
    state.session = { user: { id: "user" } };
    state.token = "token";
    state.profile = { nickname: "ランナー", nickname_public: false };
    state.loading = false;
    analytics.track.mockReset();
    analytics.save.mockReset().mockResolvedValue(undefined);
  });

  it("shows the public-name choice and warns at eleven characters", async () => {
    const view = await renderPage();
    expect(view.div.textContent).toContain("投稿にニックネームを公開する");
    expect(view.div.textContent).not.toContain("投稿詳細で");
    const input = view.div.querySelector("input:not([type='checkbox'])");
    if (!(input instanceof HTMLInputElement)) throw new Error("input missing");
    await act(async () => {
      setInput(input, "あ".repeat(10));
    });
    expect(view.div.textContent).not.toContain("10文字以内で入力してください");
    await act(async () => {
      setInput(input, "あ".repeat(11));
    });
    expect(view.div.textContent).toContain("10文字以内で入力してください");
    await view.unmount();
  });

  it.each([false, true])("counts initial profile completion only after a successful save (failure=%s)", async (failure) => {
    if (failure) analytics.save.mockRejectedValue(new Error("offline"));
    const view = await renderPage();
    try {
      const select = view.div.querySelector("select")!;
      await act(async () => {
        select.value = select.options[1].value;
        select.dispatchEvent(new Event("change", { bubbles: true }));
      });
      await act(async () => {
        view.div.querySelector("form")!.dispatchEvent(new Event("submit", { bubbles: true, cancelable: true }));
      });
      expect(analytics.save).toHaveBeenCalledTimes(1);
      if (failure) expect(analytics.track).not.toHaveBeenCalled();
      else expect(analytics.track).toHaveBeenCalledExactlyOnceWith("onboarding_completed");
    } finally {
      await view.unmount();
    }
  });

  it("does not count an existing complete profile update as onboarding", async () => {
    state.profile = { nickname: "ランナー", nickname_public: false, event: "100m" };
    const view = await renderPage();
    try {
      const select = view.div.querySelector("select")!;
      await act(async () => {
        select.value = select.options[1].value;
        select.dispatchEvent(new Event("change", { bubbles: true }));
      });
      await act(async () => {
        view.div.querySelector("form")!.dispatchEvent(new Event("submit", { bubbles: true, cancelable: true }));
      });
      expect(analytics.save).toHaveBeenCalledTimes(1);
      expect(analytics.track).not.toHaveBeenCalled();
    } finally {
      await view.unmount();
    }
  });
});
