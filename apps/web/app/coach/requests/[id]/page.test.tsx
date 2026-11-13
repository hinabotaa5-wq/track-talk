/** @vitest-environment jsdom */
import { act } from "react";
import { createRoot, type Root } from "react-dom/client";
import { afterEach, beforeEach, expect, it, vi } from "vitest";
import type { CoachReviewRequest } from "@/lib/types";
import CoachReviewRequestPage from "./page";

const apiFetch = vi.fn();
vi.mock("next/navigation", () => ({ useParams: () => ({ id: "r1" }), useRouter: () => ({ replace: vi.fn(), push: vi.fn() }) }));
vi.mock("@/lib/api", async (original) => ({ ...(await original<typeof import("@/lib/api")>()), apiFetch: (...args: unknown[]) => apiFetch(...args) }));
vi.mock("@/lib/features", async (original) => ({ ...(await original<typeof import("@/lib/features")>()), coachingMessagingEnabled: true }));
vi.mock("@/components/auth-provider", () => ({ useAuth: () => ({ session: { user: { id: "u1" } }, token: "t", loading: false }) }));
vi.mock("@/components/confirm-dialog", () => ({ useConfirm: () => async () => true, reportConfirmOptions: {} }));
vi.mock("@/components/locale-provider", () => ({ useLocale: () => "ja" }));

const base = {
  id: "r1",
  tier: "coach",
  event: "100m",
  focus: [],
  question: "スタートが遅いです",
  status: "in_review",
  started_at: "2026-11-13T12:01:00+09:00",
  due_at: "2026-11-14T12:00:00+09:00",
  created_at: "2026-11-13T12:00:00+09:00",
  info_request: null,
  withdrawal: null,
  response: null,
} as unknown as CoachReviewRequest;

const responded = {
  ...base,
  response: {
    hidden: false,
    good_point: "腕振りが良い",
    fix_points: ["接地を直す"],
    drill: "もも上げ",
    demo_video_url: null,
    created_at: "2026-11-13T13:00:00+09:00",
    clarify_reason: null,
    clarify_body: null,
    clarify_requested_at: null,
    clarification: null,
    clarified_at: null,
    helpful_asked: false,
  },
} as unknown as CoachReviewRequest;

// 追加質問の欄は返答後に出る。部屋の取得は空の履歴で答える。
const emptyRoom = {
  consultations: [],
  role: "coach",
  funding: "cash",
  price_yen: 500,
  can_purchase: false,
  block_reason: "",
  requires_video: false,
};

let root: Root;
let container: HTMLDivElement;
let scroll: ReturnType<typeof vi.fn>;
let previousScroll: typeof HTMLElement.prototype.scrollIntoView;
beforeEach(() => {
  (globalThis as typeof globalThis & { IS_REACT_ACT_ENVIRONMENT: boolean }).IS_REACT_ACT_ENVIRONMENT = true;
  container = document.createElement("div");
  document.body.appendChild(container);
  root = createRoot(container);
  apiFetch.mockReset();
  scroll = vi.fn();
  previousScroll = HTMLElement.prototype.scrollIntoView;
  HTMLElement.prototype.scrollIntoView = scroll as unknown as typeof HTMLElement.prototype.scrollIntoView;
});
afterEach(async () => {
  await act(async () => root.unmount());
  container.remove();
  HTMLElement.prototype.scrollIntoView = previousScroll;
});

async function renderPage() {
  await act(async () => root.render(<CoachReviewRequestPage />));
  await act(async () => {
    await Promise.resolve();
  });
  await act(async () => {
    await Promise.resolve();
  });
}

function submitButton(text: string): HTMLButtonElement {
  const button = [...container.querySelectorAll("button")].find((b) => b.textContent === text);
  if (!button) throw new Error(`button not found: ${text}`);
  return button as HTMLButtonElement;
}

// 返答の送信成功で、送った返答カードの先頭に完了バナーが出てスクロールする。
it("返答を送ると完了バナーが出てスクロールする", async () => {
  apiFetch.mockImplementation(async (path: string, options?: { method?: string }) => {
    if (!options?.method && path.includes("/consultations")) return emptyRoom;
    if (options?.method === "POST") return responded;
    return base;
  });
  await renderPage();
  expect(container.querySelector('[role="status"]')).toBeNull();
  await act(async () => submitButton("返答を送る").click());
  expect(container.querySelector('[role="status"]')?.textContent).toContain("返答を送りました");
  expect(scroll).toHaveBeenCalled();
  // バナーの直下に送った返答カードが続く。
  const text = container.textContent!;
  expect(text.indexOf("返答を送りました")).toBeLessThan(text.indexOf("腕振りが良い"));
});

// 返答の送信に失敗したら、バナーは出さない。
it("返答の送信に失敗したらバナーは出ない", async () => {
  apiFetch.mockImplementation(async (path: string, options?: { method?: string }) => {
    if (!options?.method && path.includes("/consultations")) return emptyRoom;
    if (options?.method === "POST") throw new Error("送信に失敗しました");
    return base;
  });
  await renderPage();
  await act(async () => submitButton("返答を送る").click());
  expect(container.querySelector('[role="status"]')).toBeNull();
  expect(container.textContent).toContain("送信に失敗しました");
  expect(scroll).not.toHaveBeenCalled();
});

it("保存済みの確認質問と補足は表示し、補足の送信フォームは出さない", async () => {
  const historical = {
    ...responded,
    status: "returned",
    response: { ...responded.response, clarify_reason: "term", clarify_body: "用語が分かりません", clarification: "補足です" },
  };
  apiFetch.mockImplementation(async (path: string) => path.includes("/consultations") ? emptyRoom : historical);
  await renderPage();
  expect(container.textContent).toContain("用語が分かりません");
  expect(container.textContent).toContain("補足です");
  expect(container.textContent).not.toContain("補足を送る");
});

// 差し戻しの送信成功で、履歴の前に完了バナーが出る。
it("差し戻すと完了バナーが履歴の前に出る", async () => {
  const sentBack = {
    ...base,
    status: "needs_info",
    info_request: {
      body: "どこが分からないか具体的に教えてください",
      reason: null,
      requested_at: "2026-11-13T13:00:00+09:00",
      from_status: "in_review",
      resubmit_by: "2026-11-14T13:00:00+09:00",
      original_question: "スタートが遅いです",
      video_replaced: false,
      resubmitted_at: null,
    },
  } as unknown as CoachReviewRequest;
  apiFetch.mockImplementation(async (path: string, options?: { method?: string }) => {
    if (!options?.method && path.includes("/consultations")) return emptyRoom;
    if (options?.method === "POST") return sentBack;
    return base;
  });
  await renderPage();
  const textarea = container.querySelector("details textarea")!;
  await act(async () => {
    Object.getOwnPropertyDescriptor(HTMLTextAreaElement.prototype, "value")!.set!.call(textarea, "どこが分からないか具体的に教えてください");
    textarea.dispatchEvent(new Event("input", { bubbles: true }));
  });
  await act(async () => submitButton("質問を付けて差し戻す").click());
  expect(container.querySelector('[role="status"]')?.textContent).toContain("差し戻しました");
  expect(scroll).toHaveBeenCalled();
  const text = container.textContent!;
  expect(text.indexOf("差し戻しました")).toBeLessThan(text.indexOf("差し戻し中"));
});

it("添削をはじめると入力欄を表示する", async () => {
  apiFetch.mockImplementation(async (path: string, options?: { method?: string }) => {
    if (!options?.method && path.includes("/consultations")) return emptyRoom;
    return options?.method === "POST" ? base : { ...base, started_at: null };
  });
  await renderPage();
  expect(container.textContent).not.toContain("返答を送る");
  await act(async () => submitButton("添削をはじめる").click());
  expect(apiFetch).toHaveBeenCalledWith("/v1/coach/requests/r1/start", { method: "POST", token: "t" });
  expect(container.textContent).toContain("返答を送る");
});
