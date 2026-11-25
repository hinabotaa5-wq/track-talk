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
    demo_video_url: null,
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

// B3：行ごとに種目・区分・相談内容・返却日時・動画、詳細へのリンク。
it("返却済みの添削を1件ずつ、見出し・相談内容・返却日時・動画・詳細へのリンク付きで並べる（B3）", async () => {
  mockCompleted([
    request({ id: "r1" }),
    request({ id: "r2", event: "long-jump", question: "踏み切りを見てください", returned_at: "2026-11-23T05:30:00Z", video_url: "https://video.example/r2.mp4", thumbnail_url: "https://video.example/r2.jpg" }),
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
    const video = items[0].querySelector<HTMLVideoElement>('[data-testid="history-video"]')!;
    expect(video.getAttribute("src")).toBe("https://video.example/r1.mp4");
    expect(video.getAttribute("poster")).toBe("https://video.example/r1.jpg");
    expect(video.hasAttribute("controls")).toBe(true);
    expect(video.hasAttribute("playsinline")).toBe(true);
    expect(video.getAttribute("preload")).toBe("metadata");
    expect(items[1].querySelector('[data-testid="history-video"]')?.getAttribute("src")).toBe("https://video.example/r2.mp4");
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
    expect(clarifying.querySelector('[data-testid="history-video"]')).not.toBeNull();
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

// B9・B13：動画なし・削除済み。
it("動画なしの行はプレーヤーも注記も出さず、削除済みの行はプレーヤーを出さずに注記を1つだけ出す（B9・B13）", async () => {
  mockCompleted([
    request({ id: "none", video_url: undefined, thumbnail_url: undefined }),
    request({ id: "purged", video_url: undefined, thumbnail_url: undefined, videos_purged: true, demo_video_url: null }),
  ]);
  const { container, root } = await renderPage();
  try {
    const [none, purged] = rows(container);
    expect(none.querySelector("video")).toBeNull();
    expect(none.textContent).not.toContain("削除しました");
    expect(purged.querySelector("video")).toBeNull();
    expect(purged.textContent?.match(/削除しました/g)).toHaveLength(1);
    expect(purged.textContent).toContain("動画は保存期間（添削のやりとりが終わってから7日）を過ぎたため削除しました。");
  } finally {
    await teardown(root, container);
  }
});

// B10・B11：見本動画。
it("見本動画がある行は「見本動画」のラベル付きでプレーヤーを出し、null の行では出さない（B10・B11）", async () => {
  mockCompleted([
    request({ id: "demo", demo_video_url: "https://video.example/demo.mp4" }),
    request({ id: "plain", demo_video_url: null }),
  ]);
  const { container, root } = await renderPage();
  try {
    const [demo, plain] = rows(container);
    const demoVideo = demo.querySelector<HTMLVideoElement>('[data-testid="history-demo-video"]')!;
    expect(demoVideo.getAttribute("src")).toBe("https://video.example/demo.mp4");
    expect(demoVideo.closest("figure")?.querySelector("figcaption")?.textContent).toBe("見本動画");
    expect(demo.querySelector('[data-testid="history-video"]')).not.toBeNull();
    expect(plain.querySelector('[data-testid="history-demo-video"]')).toBeNull();
    expect(plain.textContent).not.toContain("見本動画");
  } finally {
    await teardown(root, container);
  }
});

// B12：ミニプレーヤー（幅160px・高さ220pxが上限、縦横比を保つ、行幅いっぱいにしない）。2つは横に並べて折り返せる。
it("選手の動画・見本動画とも、幅160px・高さ220pxを上限にしたミニプレーヤーで出す（B12）", async () => {
  mockCompleted([request({ id: "demo", demo_video_url: "https://video.example/demo.mp4" })]);
  const { container, root } = await renderPage();
  try {
    const videos = [...container.querySelectorAll("video")];
    expect(videos).toHaveLength(2);
    for (const video of videos) {
      const classes = video.className.split(/\s+/);
      expect(classes).toContain("max-w-[160px]");
      expect(classes).toContain("max-h-[220px]");
      expect(classes).toContain("object-contain");
      expect(classes).toContain("bg-black");
      expect(classes).not.toContain("w-full");
    }
    const wrap = videos[0].parentElement!;
    expect(wrap.className.split(/\s+/)).toEqual(expect.arrayContaining(["flex", "flex-wrap"]));
    expect(wrap.contains(videos[1])).toBe(true);
  } finally {
    await teardown(root, container);
  }
});

it("英語では History・Returned・Open と出す", async () => {
  const { installEnglish, setActiveLocale } = await import("@/lib/i18n");
  installEnglish(en);
  setActiveLocale("en");
  mockCompleted([request({ id: "r1", demo_video_url: "https://video.example/demo.mp4" })]);
  const { container, root } = await renderPage();
  try {
    expect(container.querySelector("h1")?.textContent).toBe("History");
    expect(container.textContent).toContain("Returned ");
    expect(container.querySelector("a")?.textContent).toBe("Open");
    expect(container.querySelector("figcaption")?.textContent).toBe("Demo video");
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
