/** @vitest-environment jsdom */
import { act } from "react";
import { createRoot, type Root } from "react-dom/client";
import { expect, it, vi } from "vitest";
import { en } from "@/lib/i18n-en";
import { formatDateTime } from "@/lib/format";
import type { CoachReviewRequest } from "@/lib/types";
import CoachHistoryPage from "./page";

const replace = vi.fn();
vi.mock("next/navigation", () => ({ useRouter: () => ({ replace, push: vi.fn() }) }));
const auth = vi.hoisted(() => ({ session: { user: { id: "coach" } } as unknown, token: "token" as string | null, loading: false }));
vi.mock("@/components/auth-provider", () => ({ useAuth: () => auth }));
vi.mock("@/lib/features", async (original) => ({
  ...await original<typeof import("@/lib/features")>(),
  coachingEnabled: true,
}));
const apiFetch = vi.fn();
vi.mock("@/lib/api", async (original) => ({
  ...await original<typeof import("@/lib/api")>(),
  apiFetch: (...args: unknown[]) => apiFetch(...args),
}));

function request(overrides: Partial<CoachReviewRequest>): CoachReviewRequest {
  return {
    id: "r1",
    tier: "coach",
    event: "100m",
    focus: [],
    question: "スタートを見てください",
    status: "returned",
    due_at: "2026-11-24T00:00:00Z",
    created_at: "2026-11-23T00:00:00Z",
    returned_at: "2026-11-24T03:00:00Z",
    video_url: "https://video.example/r1.mp4",
    thumbnail_url: "https://video.example/r1.jpg",
    videos_purged: false,
    ...overrides,
  };
}

function mockCompleted(completed: CoachReviewRequest[]) {
  apiFetch.mockImplementation(async (path: string) => {
    if (path !== "/v1/coach/requests") throw new Error(`Unexpected request: ${path}`);
    return { offers: [], reviewing: [], completed };
  });
}

async function renderPage() {
  vi.stubGlobal("IS_REACT_ACT_ENVIRONMENT", true);
  const container = document.createElement("div");
  document.body.appendChild(container);
  const root = createRoot(container);
  await act(async () => root.render(<CoachHistoryPage />));
  return { container, root };
}

async function teardown(root: Root, container: HTMLDivElement) {
  await act(async () => root.unmount());
  container.remove();
  vi.unstubAllGlobals();
  vi.clearAllMocks();
  auth.session = { user: { id: "coach" } };
  auth.token = "token";
}

function rows(container: HTMLElement) {
  return [...container.querySelectorAll("li")];
}

// B3：行ごとに種目・区分・相談内容・返却日時、詳細へのリンク。
it("返却済みの添削を1件ずつ、見出し・相談内容・返却日時・詳細へのリンク付きで並べる（B3）", async () => {
  mockCompleted([
    request({ id: "r1" }),
    request({ id: "r2", event: "long-jump", question: "踏み切りを見てください", returned_at: "2026-11-23T05:30:00Z" }),
  ]);
  const { container, root } = await renderPage();
  try {
    expect(apiFetch).toHaveBeenCalledWith("/v1/coach/requests", { token: "token" });
    expect(apiFetch).toHaveBeenCalledTimes(1);
    expect(container.querySelector("h1")?.textContent).toBe("履歴");
    const items = rows(container);
    expect(items).toHaveLength(2);
    expect(items[0].textContent).toContain("100m · コーチ");
    expect(items[0].textContent).toContain("スタートを見てください");
    expect(items[0].textContent).toContain(`返却 ${formatDateTime("2026-11-24T03:00:00Z")}`);
    expect(items[1].textContent).toContain("踏み切りを見てください");
    expect(items[1].textContent).toContain(`返却 ${formatDateTime("2026-11-23T05:30:00Z")}`);
    expect(items.map((li) => li.querySelector("a")?.getAttribute("href"))).toEqual(["/coach/requests/r1", "/coach/requests/r2"]);
    expect(items[0].querySelector("a")?.textContent).toBe("開く");
    expect(container.textContent).not.toContain("まだ返却した添削はありません。");
  } finally {
    await teardown(root, container);
  }
});

// B4：0件の案内。
it("0件のときは「まだ返却した添削はありません。」を出し、行は出さない（B4）", async () => {
  mockCompleted([]);
  const { container, root } = await renderPage();
  try {
    expect(container.textContent).toContain("まだ返却した添削はありません。");
    expect(rows(container)).toHaveLength(0);
  } finally {
    await teardown(root, container);
  }
});

// B5：clarifying も同じ形で、状態のバッジは出さない。
it("補足回答待ち（clarifying）の行も同じ形で出し、特別なバッジは付けない（B5）", async () => {
  mockCompleted([request({ id: "r1" }), request({ id: "r2", status: "clarifying" })]);
  const { container, root } = await renderPage();
  try {
    const [returned, clarifying] = rows(container);
    expect(clarifying.querySelector("a")?.getAttribute("href")).toBe("/coach/requests/r2");
    expect(clarifying.querySelector("a")?.textContent).toBe("開く");
    expect(clarifying.querySelectorAll("span")).toHaveLength(returned.querySelectorAll("span").length);
    expect(clarifying.textContent).not.toMatch(/補足|確認待ち|clarifying/);
  } finally {
    await teardown(root, container);
  }
});

// B6：読み込み失敗。
it("読み込めなかったときはエラーを出す（B6）", async () => {
  apiFetch.mockRejectedValue(new Error("network"));
  const { container, root } = await renderPage();
  try {
    expect(container.querySelector('[role="alert"]')?.textContent).toBe("履歴を読み込めませんでした");
    expect(rows(container)).toHaveLength(0);
  } finally {
    await teardown(root, container);
  }
});

// B7：未ログイン。
it("未ログインのときは /login?next=%2Fcoach%2Fhistory へ送る（B7）", async () => {
  auth.session = null;
  auth.token = null;
  const { container, root } = await renderPage();
  try {
    expect(replace).toHaveBeenCalledWith("/login?next=%2Fcoach%2Fhistory");
    expect(apiFetch).not.toHaveBeenCalled();
  } finally {
    await teardown(root, container);
  }
});

// 一覧には動画を出さない（運営者決定 2026-11-25）。動画の URL があっても、削除済みでも、プレーヤーも注記も出さない。
it("一覧には動画・見本動画・削除済みの注記を出さない", async () => {
  mockCompleted([
    request({ id: "with-video" }),
    request({ id: "purged", video_url: undefined, thumbnail_url: undefined, videos_purged: true }),
  ]);
  const { container, root } = await renderPage();
  try {
    expect(rows(container)).toHaveLength(2);
    expect(container.querySelector("video")).toBeNull();
    expect(container.querySelector("img")).toBeNull();
    expect(container.textContent).not.toContain("見本動画");
    expect(container.textContent).not.toContain("削除しました");
  } finally {
    await teardown(root, container);
  }
});

it("英語では History・Returned・Open と出す", async () => {
  const { installEnglish, setActiveLocale } = await import("@/lib/i18n");
  installEnglish(en);
  setActiveLocale("en");
  mockCompleted([request({ id: "r1" })]);
  const { container, root } = await renderPage();
  try {
    expect(container.querySelector("h1")?.textContent).toBe("History");
    expect(container.textContent).toContain("Returned ");
    expect(container.querySelector("a")?.textContent).toBe("Open");
  } finally {
    setActiveLocale("ja");
    await teardown(root, container);
  }
});

it("英語で0件のときの案内", async () => {
  const { installEnglish, setActiveLocale } = await import("@/lib/i18n");
  installEnglish(en);
  setActiveLocale("en");
  mockCompleted([]);
  const { container, root } = await renderPage();
  try {
    expect(container.textContent).toContain("You haven't returned any reviews yet.");
  } finally {
    setActiveLocale("ja");
    await teardown(root, container);
  }
});
