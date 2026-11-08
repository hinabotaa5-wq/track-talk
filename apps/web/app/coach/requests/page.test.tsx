/** @vitest-environment jsdom */
import { act, type ReactNode } from "react";
import { createRoot, type Root } from "react-dom/client";
import { afterEach, beforeEach, expect, it, vi } from "vitest";
import CoachRequestsPage from "./page";

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

it("依頼一覧は/coachへ戻せて、引き受け・見送りができる", async () => {
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
  await act(async () => root.render(<CoachRequestsPage />));
  expect(container.querySelector('a[href="/coach"]')?.textContent).toContain("コーチ画面に戻る");
  expect(container.textContent).toContain("新しい依頼が入ってから90分以内");
  expect(container.textContent).toContain("引き受ける");
  expect(container.textContent).toContain("見送る");

  await act(async () => container.querySelector("button")!.dispatchEvent(new Event("click", { bubbles: true })));
  expect(apiFetch.mock.calls.some(([path, options]) => path === "/v1/coach/offers/o1/accept" && options?.method === "POST")).toBe(true);
});
