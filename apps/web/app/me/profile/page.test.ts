/** @vitest-environment jsdom */
import { createElement } from "react";
import { act } from "react";
import { createRoot, type Root } from "react-dom/client";
import { beforeEach, describe, expect, it, vi } from "vitest";
import { ApiRequestError } from "@/lib/api";
import { reservedNicknameMessage } from "@/lib/reserved-nickname";
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
      is_admin?: boolean;
      birth_year_known?: boolean;
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
      const select = view.div.querySelector<HTMLSelectElement>("select[name='event']")!;
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

  it.each([
    { current: null, nickname: "ａｄｍｉｎ", admin: false, blocked: true },
    { current: "ランナー", nickname: "公式戦運営", admin: false, blocked: true },
    { current: "ランナー", nickname: "公式戦がんばる", admin: false, blocked: false },
    { current: "運営", nickname: "運営", admin: false, blocked: false },
    { current: "ランナー", nickname: "運営", admin: true, blocked: false },
  ])("validates a nickname from $current to $nickname (admin=$admin)", async ({ current, nickname, admin, blocked }) => {
    state.profile = { nickname: current, nickname_public: false, event: "100m", is_admin: admin };
    const view = await renderPage();
    try {
      await act(async () => {
        setInput(view.div.querySelector("input[placeholder='ニックネーム']")!, nickname);
      });
      await act(async () => {
        view.div.querySelector("form")!.dispatchEvent(new Event("submit", { bubbles: true, cancelable: true }));
      });
      if (blocked) {
        expect(analytics.save).not.toHaveBeenCalled();
        expect(view.div.querySelector('[role="alert"]')?.textContent).toBe(reservedNicknameMessage);
      } else {
        expect(analytics.save).toHaveBeenCalledTimes(1);
      }
    } finally {
      await view.unmount();
    }
  });

  it("shows the API's reserved nickname error when the displayed role is stale", async () => {
    state.profile = { nickname: "ランナー", nickname_public: false, event: "100m", is_admin: true };
    analytics.save.mockRejectedValue(new ApiRequestError(422, {
      error: { code: "nickname_reserved", message: reservedNicknameMessage },
    }));
    const view = await renderPage();
    try {
      await act(async () => {
        setInput(view.div.querySelector("input[placeholder='ニックネーム']")!, "運営");
      });
      await act(async () => {
        view.div.querySelector("form")!.dispatchEvent(new Event("submit", { bubbles: true, cancelable: true }));
      });
      expect(view.div.querySelector('[role="alert"]')?.textContent).toBe(reservedNicknameMessage);
    } finally {
      await view.unmount();
    }
  });

  it("does not count an existing complete profile update as onboarding", async () => {
    state.profile = { nickname: "ランナー", nickname_public: false, event: "100m" };
    const view = await renderPage();
    try {
      const select = view.div.querySelector<HTMLSelectElement>("select[name='event']")!;
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

  // 生まれた年（本体 #1045 の T1〜T4）。
  async function submit(view: { div: HTMLDivElement }) {
    await act(async () => {
      view.div.querySelector("form")!.dispatchEvent(new Event("submit", { bubbles: true, cancelable: true }));
    });
  }
  const yearInput = (div: HTMLDivElement) => div.querySelector<HTMLInputElement>("input[inputmode='numeric']");

  it("年の情報がない人には、種目の下・公開チェックの上に生まれた年の欄を出す", async () => {
    state.profile = { nickname: null, nickname_public: false, birth_year_known: false };
    const view = await renderPage();
    try {
      const nickname = view.div.querySelector("input[placeholder='ニックネーム']")!;
      const event = view.div.querySelector("select[name='event']")!;
      const year = yearInput(view.div)!;
      const publicBox = view.div.querySelector("input[type='checkbox']")!;
      const save = view.div.querySelector("button[type='submit']")!;
      expect(year).not.toBeNull();
      expect(year.closest("label")?.textContent).toContain("生まれた年");
      const order = [nickname, event, year, publicBox, save];
      for (let i = 1; i < order.length; i++) {
        expect(order[i - 1].compareDocumentPosition(order[i]) & Node.DOCUMENT_POSITION_FOLLOWING).toBeTruthy();
      }
    } finally {
      await view.unmount();
    }
  });

  it.each([
    { value: "", message: "生まれた年を入力してください" },
    { value: "20", message: "生まれた年は西暦4桁で入力してください" },
    { value: "abcd", message: "生まれた年は西暦4桁で入力してください" },
    { value: "1899", message: "生まれた年が正しくありません" },
    { value: "2999", message: "生まれた年が正しくありません" },
  ])("生まれた年が「$value」なら保存せず理由を出す", async ({ value, message }) => {
    state.profile = { nickname: "ランナー", nickname_public: false, event: "100m", birth_year_known: false };
    const view = await renderPage();
    try {
      await act(async () => {
        setInput(yearInput(view.div)!, value);
      });
      await submit(view);
      expect(analytics.save).not.toHaveBeenCalled();
      expect(view.div.querySelector('[role="alert"]')?.textContent).toBe(message);
    } finally {
      await view.unmount();
    }
  });

  it("生まれた年を数値で送り、年だけがまだの既存の人は表示言語を上書きしない", async () => {
    state.profile = { nickname: "ランナー", nickname_public: false, event: "100m", birth_year_known: false };
    const view = await renderPage();
    try {
      expect(view.div.textContent).toContain("生まれた年を登録してください");
      await act(async () => {
        setInput(yearInput(view.div)!, "２００８");
      });
      await submit(view);
      expect(analytics.save).toHaveBeenCalledTimes(1);
      const body = JSON.parse(analytics.save.mock.calls[0][1].body);
      expect(body.birth_year).toBe(2008);
      expect(body).not.toHaveProperty("locale");
    } finally {
      await view.unmount();
    }
  });

  it("登録直後は、ニックネーム・種目・生まれた年と表示言語をまとめて送る", async () => {
    state.profile = { nickname: null, nickname_public: false, birth_year_known: false };
    const view = await renderPage();
    try {
      await act(async () => {
        setInput(view.div.querySelector("input[placeholder='ニックネーム']")!, "はしる");
        const select = view.div.querySelector<HTMLSelectElement>("select[name='event']")!;
        select.value = select.options[1].value;
        select.dispatchEvent(new Event("change", { bubbles: true }));
        setInput(yearInput(view.div)!, "2010");
      });
      await submit(view);
      expect(analytics.save).toHaveBeenCalledTimes(1);
      const body = JSON.parse(analytics.save.mock.calls[0][1].body);
      expect(body).toMatchObject({ nickname: "はしる", birth_year: 2010 });
      expect(body).toHaveProperty("locale");
    } finally {
      await view.unmount();
    }
  });

  it("年を登録済み（旧方式の生年月を含む）の人には欄を出さず、年も送らない", async () => {
    state.profile = { nickname: "ランナー", nickname_public: false, event: "100m", birth_year_known: true };
    const view = await renderPage();
    try {
      expect(yearInput(view.div)).toBeNull();
      expect(view.div.textContent).not.toContain("生まれた年");
      await submit(view);
      expect(analytics.save).toHaveBeenCalledTimes(1);
      expect(JSON.parse(analytics.save.mock.calls[0][1].body)).not.toHaveProperty("birth_year");
    } finally {
      await view.unmount();
    }
  });
});
