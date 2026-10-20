/** @vitest-environment jsdom */
import { createElement } from "react";
import { act } from "react";
import { createRoot, type Root } from "react-dom/client";
import { beforeEach, describe, expect, it, vi } from "vitest";
import BadgePage from "./page";

const { state, apiFetch } = vi.hoisted(() => ({
  state: {
    session: { user: { id: "user" } } as { user: { id: string } } | null,
    token: "token" as string | null,
    profile: { nickname: "ランナー", nickname_public: true, post_master_badge_public: true } as {
      nickname: string | null;
      nickname_public: boolean;
      post_master_badge_public?: boolean;
    } | null,
    loading: false,
  },
  apiFetch: vi.fn(),
}));

vi.mock("next/navigation", () => ({
  useRouter: () => ({ replace: vi.fn() }),
}));

vi.mock("@/components/auth-provider", () => ({
  useAuth: () => ({ ...state, refreshProfile: vi.fn() }),
}));

vi.mock("@/lib/api", () => ({
  apiFetch,
  ApiRequestError: class extends Error {},
}));

async function renderPage() {
  const div = document.createElement("div");
  document.body.appendChild(div);
  let root: Root;
  await act(async () => {
    root = createRoot(div);
    root.render(createElement(BadgePage));
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

describe("BadgePage", () => {
  beforeEach(() => {
    state.profile = { nickname: "ランナー", nickname_public: true, post_master_badge_public: true };
    apiFetch.mockReset();
    apiFetch.mockResolvedValue({});
  });

  it("hides the badge when unchecked", async () => {
    const view = await renderPage();
    expect(view.div.textContent).toContain("「投稿マスター」のバッジを表示する");
    expect(view.div.textContent).not.toContain("ニックネームの変更");
    const box = view.div.querySelector("input[type='checkbox']");
    if (!(box instanceof HTMLInputElement)) throw new Error("checkbox missing");
    expect(box.checked).toBe(true);
    await act(async () => {
      box.click();
    });
    expect(apiFetch).toHaveBeenCalledWith("/v1/me/badge", expect.objectContaining({
      method: "PATCH",
      body: JSON.stringify({ post_master_badge_public: false }),
    }));
    expect(view.div.textContent).toContain("バッジを隠しました。");
    await view.unmount();
  });
});
