/** @vitest-environment jsdom */
import { createElement } from "react";
import { act } from "react";
import { createRoot, type Root } from "react-dom/client";
import { beforeEach, describe, expect, it, vi } from "vitest";
import DeleteAccountPage from "./page";
import { installEnglish, setActiveLocale } from "@/lib/i18n";
import { en } from "@/lib/i18n-en";

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

  it("shows the coaching notice in theme-aware body text (readable in light mode)", async () => {
    apiFetch.mockResolvedValueOnce({ application: { status: "approved" } });
    apiFetch.mockResolvedValue({ auth_deleted: true });
    const view = await renderPage();
    await act(async () => {});
    const notice = Array.from(view.div.querySelectorAll("p")).find((p) =>
      p.textContent?.includes("コーチだけをやめたいときは"),
    );
    if (!notice) throw new Error("coaching notice missing");
    // 明表示では text-pure-white（常時 #fff）が淡い背景に埋もれるため、
    // ページ本文と同じテーマ対応の文字色を使うこと。
    expect(notice.className).not.toContain("text-pure-white");
    expect(notice.className).toContain("text-slate-300");
    const link = notice.querySelector('a[href="/coach/withdraw"]');
    if (!link) throw new Error("withdraw link missing");
    expect(link.className).toContain("text-blue-300");
    await view.unmount();
  });

  it("退会画面は退会の前提つきで案内し、無条件の旧文を残さない（#717 残件1）", async () => {
    apiFetch.mockResolvedValue({ auth_deleted: true });
    const view = await renderPage();
    await act(async () => {});
    const text = view.div.textContent ?? "";
    // (a) 進行中・未解決の対応がある間は退会できない。(b) 削除はその後。(c) 取引記録は保持。
    expect(text).toContain("終わっていないときは退会できません");
    expect(text).toContain("対応が終わったあとに退会すると、あなたの指導の契約とやりとりも削除されます");
    expect(text).toContain("取引に関する記録はプライバシーポリシーに定める期間は退会後も保管します");
    expect(text).not.toContain("オンラインコーチングの提供中は、あなたの指導の契約とやりとりも削除されます");
    await view.unmount();
  });

  it("退会画面の英語も前提つきで、旧文を残さない（#717 残件1）", async () => {
    installEnglish(en);
    setActiveLocale("en");
    try {
      apiFetch.mockResolvedValue({ auth_deleted: true });
      const view = await renderPage();
      await act(async () => {});
      const text = view.div.textContent ?? "";
      expect(text).toContain("you cannot delete your account until they are done");
      expect(text).toContain("deleting your account also deletes your coaching contracts and messages");
      expect(text).toContain("Transaction records are kept after deletion");
      expect(text).not.toContain("If you're providing online coaching");
      await view.unmount();
    } finally {
      setActiveLocale("ja");
    }
  });

  it("keeps the withdraw link style and wording unchanged", async () => {
    apiFetch.mockResolvedValueOnce({ application: { status: "approved" } });
    apiFetch.mockResolvedValue({ auth_deleted: true });
    const view = await renderPage();
    await act(async () => {});
    const notice = Array.from(view.div.querySelectorAll("p")).find((p) =>
      p.textContent?.includes("コーチだけをやめたいときは"),
    );
    if (!notice) throw new Error("coaching notice missing");
    // リンクは青文字＋下線のまま、文面は変えない（#468 の「やらないこと」の回帰網）。
    const link = notice.querySelector("a");
    expect(link?.getAttribute("href")).toBe("/coach/withdraw");
    expect(link?.className).toContain("text-blue-300");
    expect(link?.className).toContain("underline");
    expect(notice.textContent).toContain("送金が済んでから退会してください。");
    await view.unmount();
  });
});
