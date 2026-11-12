/** @vitest-environment jsdom */
import { act, type ReactNode } from "react";
import { createRoot, type Root } from "react-dom/client";
import { afterEach, beforeEach, expect, it, vi } from "vitest";
import CoachOffersPage from "./page";
import CoachReviewingPage from "../reviewing/page";

const apiFetch = vi.fn();
vi.mock("@/lib/features", async (original) => ({
  ...await original<typeof import("@/lib/features")>(),
  coachingEnabled: true,
}));
vi.mock("@/lib/api", async (original) => ({
  ...await original<typeof import("@/lib/api")>(),
  apiFetch: (...args: unknown[]) => apiFetch(...args),
}));
vi.mock("@/components/auth-provider", () => ({
  useAuth: () => ({ session: {}, profile: null, token: "test", loading: false }),
}));
vi.mock("next/navigation", () => ({ useRouter: () => ({ replace: vi.fn() }) }));
vi.mock("next/link", () => ({
  default: ({ href, children, ...props }: { href: string; children: ReactNode }) => (
    <a href={href} {...props}>{children}</a>
  ),
}));

let root: Root;
let container: HTMLDivElement;
beforeEach(() => {
  (globalThis as typeof globalThis & { IS_REACT_ACT_ENVIRONMENT: boolean }).IS_REACT_ACT_ENVIRONMENT = true;
  apiFetch.mockReset();
  container = document.createElement("div");
  document.body.appendChild(container);
  root = createRoot(container);
});
afterEach(async () => {
  await act(async () => root.unmount());
  container.remove();
});

it("届いた依頼の画面は届いた依頼だけを出し、引き受け・見送りができる", async () => {
  const request = {
    id: "r1",
    event: "100m",
    tier: "coach",
    question: "フォームを見てください",
    focus: [],
    status: "offered",
    due_at: "2026-12-01T00:00:00Z",
    requested_coach_id: "coach-1",
    coach_reward_yen: 1050,
  };
  apiFetch.mockImplementation((path: string) =>
    path === "/v1/coach/requests"
      ? Promise.resolve({
        offers: [{ id: "o1", expires_at: new Date(Date.now() + 60 * 60000).toISOString(), request }],
        reviewing: [],
      })
      : Promise.reject(new Error(`unexpected ${path}`)),
  );
  await act(async () => root.render(<CoachOffersPage />));
  // 戻る導線はヘッダーの「＜」だけ。画面内に文字リンクは置かない。
  expect(container.querySelector('a[href="/coach"]')).toBeNull();
  expect(container.textContent).not.toContain("コーチ画面に戻る");
  expect(container.querySelector("h1")?.textContent).toBe("届いた依頼");
  expect(container.textContent).not.toContain("添削中の依頼はありません");
  expect(container.textContent).not.toContain("新しい依頼が入ってから90分以内");
  expect(container.textContent).toContain("※23:00–07:00はカウント停止");
  expect(container.textContent).toContain("引き受ける");
  expect(container.textContent).toContain("見送る");
  // #592: 指名ありでも「あなたを選んだ依頼」行は出さない（両リスト共用）。
  expect(container.textContent).not.toContain("あなたを選んだ依頼");

  await act(async () => container.querySelector("button")!.dispatchEvent(new Event("click", { bubbles: true })));
  expect(apiFetch.mock.calls.some(([path, options]) => path === "/v1/coach/offers/o1/accept" && options?.method === "POST")).toBe(true);
});

const reviewingRequest = {
  id: "r2",
  event: "100m",
  tier: "coach",
  question: "スタートを見てください",
  focus: [],
  status: "in_review",
  due_at: "2026-12-01T00:00:00Z",
  requested_coach_id: "coach-1",
  coach_reward_yen: 1050,
};

it("添削中の画面は添削中だけを出し、90分の説明文と引き受けボタンは出さない", async () => {
  apiFetch.mockImplementation((path: string) =>
    path === "/v1/coach/requests"
      ? Promise.resolve({
        offers: [{ id: "o1", expires_at: new Date(Date.now() + 60 * 60000).toISOString(), request: { ...reviewingRequest, id: "r1", status: "offered" } }],
        reviewing: [reviewingRequest],
      })
      : Promise.reject(new Error(`unexpected ${path}`)),
  );
  await act(async () => root.render(<CoachReviewingPage />));
  expect(container.querySelector("h1")?.textContent).toBe("添削中");
  expect(container.textContent).toContain("スタートを見てください");
  expect(container.querySelector('a[href="/coach/requests/r2"]')).not.toBeNull();
  expect(container.textContent).not.toContain("新しい依頼が入ってから90分以内");
  expect(container.textContent).not.toContain("引き受ける");
  expect(container.textContent).not.toContain("いま届いている依頼はありません");
  // #592: 添削中も指名ありで「あなたを選んだ依頼」行は出さない。
  expect(container.textContent).not.toContain("あなたを選んだ依頼");
  expect(container.querySelector('a[href="/coach"]')).toBeNull();
});

it("どちらも空のときは、それぞれの空表示だけが出る", async () => {
  apiFetch.mockResolvedValue({ offers: [], reviewing: [] });
  await act(async () => root.render(<CoachOffersPage />));
  expect(container.textContent).toContain("いま届いている依頼はありません");
  expect(container.textContent).not.toContain("添削中の依頼はありません");
  await act(async () => root.render(<CoachReviewingPage />));
  expect(container.textContent).toContain("添削中の依頼はありません");
  expect(container.textContent).not.toContain("いま届いている依頼はありません");
});
