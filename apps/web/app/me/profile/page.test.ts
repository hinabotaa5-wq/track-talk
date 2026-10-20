/** @vitest-environment jsdom */
import { createElement } from "react";
import { act } from "react";
import { createRoot, type Root } from "react-dom/client";
import { beforeEach, describe, expect, it, vi } from "vitest";
import ProfilePage from "./page";

const { state } = vi.hoisted(() => ({
  state: {
    session: { user: { id: "user" } } as { user: { id: string } } | null,
    token: "token" as string | null,
    profile: { nickname: "ランナー", nickname_public: false } as {
      nickname: string | null;
      nickname_public: boolean;
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
});
