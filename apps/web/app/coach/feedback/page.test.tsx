/** @vitest-environment jsdom */
import { act } from "react";
import { createRoot, type Root } from "react-dom/client";
import { expect, it, vi } from "vitest";
import { en } from "@/lib/i18n-en";
import CoachFeedbackPage from "./page";

const replace = vi.fn();
vi.mock("next/navigation", () => ({ useRouter: () => ({ replace, push: vi.fn() }) }));
vi.mock("@/components/auth-provider", () => ({
  useAuth: () => ({ session: { user: { id: "coach" } }, token: "token", loading: false }),
}));
vi.mock("@/lib/features", async (original) => ({
  ...await original<typeof import("@/lib/features")>(),
  coachingEnabled: true,
}));
const apiFetch = vi.fn();
vi.mock("@/lib/api", async (original) => ({
  ...await original<typeof import("@/lib/api")>(),
  apiFetch: (...args: unknown[]) => apiFetch(...args),
}));

async function renderPage() {
  vi.stubGlobal("IS_REACT_ACT_ENVIRONMENT", true);
  const container = document.createElement("div");
  document.body.appendChild(container);
  const root = createRoot(container);
  await act(async () => root.render(<CoachFeedbackPage />));
  return { container, root };
}

async function teardown(root: Root, container: HTMLDivElement) {
  await act(async () => root.unmount());
  container.remove();
  vi.unstubAllGlobals();
  vi.clearAllMocks();
}

// C1: 感想の本文だけが並び、日付・選手名・依頼へのリンクは出さない。
it("一覧には感想の本文だけを並べ、日付・選手・依頼へのリンクを出さない", async () => {
  apiFetch.mockImplementation(async (path: string) => {
    if (path !== "/v1/coach/feedback") throw new Error(`Unexpected request: ${path}`);
    return { feedbacks: [{ id: "f1", body: "とても分かりやすかったです" }, { id: "f2", body: "ドリルを続けます\n2行目" }] };
  });
  const { container, root } = await renderPage();
  try {
    expect(apiFetch).toHaveBeenCalledWith("/v1/coach/feedback", { token: "token" });
    expect(container.querySelector("h1")?.textContent).toBe("選手からの感想");
    const items = Array.from(container.querySelectorAll("li")).map((li) => li.textContent);
    expect(items).toEqual(["とても分かりやすかったです", "ドリルを続けます\n2行目"]);
    expect(container.querySelectorAll("a")).toHaveLength(0);
    expect(container.querySelector("time")).toBeNull();
    expect(container.textContent).not.toMatch(/\d{4}[/年-]\d{1,2}|\d{1,2}月\d{1,2}日|\d{1,2}:\d{2}/);
    expect(container.textContent).not.toContain("まだ感想はありません。");
  } finally {
    await teardown(root, container);
  }
});

// C2: 0件のときの案内。
it("0件のときは「まだ感想はありません。」を出す", async () => {
  apiFetch.mockResolvedValue({ feedbacks: [] });
  const { container, root } = await renderPage();
  try {
    expect(container.textContent).toContain("まだ感想はありません。");
    expect(container.querySelectorAll("li")).toHaveLength(0);
  } finally {
    await teardown(root, container);
  }
});

it("読み込めなかったときはエラーを出す", async () => {
  apiFetch.mockRejectedValue(new Error("network"));
  const { container, root } = await renderPage();
  try {
    expect(container.querySelector('[role="alert"]')?.textContent).toBe("感想を読み込めませんでした");
  } finally {
    await teardown(root, container);
  }
});

it("新しい文言には英訳がある", () => {
  for (const key of ["選手からの感想", "まだ感想はありません。", "感想を読み込めませんでした"]) {
    expect(en[key]).toBeTruthy();
  }
});
