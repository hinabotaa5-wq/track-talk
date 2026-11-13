/** @vitest-environment jsdom */
import { act } from "react";
import { createRoot } from "react-dom/client";
import { expect, it, vi } from "vitest";
import ReviewRequestPage from "./page";

const apiFetch = vi.fn();
vi.mock("next/navigation", () => ({
  useParams: () => ({ id: "review-1" }),
  useRouter: () => ({ replace: vi.fn() }),
  useSearchParams: () => new URLSearchParams(),
}));
vi.mock("@/components/auth-provider", () => ({
  useAuth: () => ({ session: { user: { id: "athlete" } }, token: "token", loading: false }),
}));
vi.mock("@/components/confirm-dialog", () => ({
  useConfirm: () => async () => true,
  reportConfirmOptions: (value: unknown) => value,
}));
vi.mock("@/lib/api", async (original) => ({
  ...await original<typeof import("@/lib/api")>(),
  apiFetch: (...args: unknown[]) => apiFetch(...args),
}));
vi.mock("@/lib/features", async (original) => ({
  ...await original<typeof import("@/lib/features")>(),
  coachingMessagingEnabled: true,
  coachingConsultationEnabled: true,
}));

it("返却後の質問には有料の追加質問だけを表示する", async () => {
  vi.stubGlobal("IS_REACT_ACT_ENVIRONMENT", true);
  const container = document.createElement("div");
  document.body.appendChild(container);
  const root = createRoot(container);
  apiFetch.mockImplementation(async (path: string) => {
    if (path.endsWith("/consultations")) {
      return { consultations: [], role: "requester", funding: "cash", price_yen: 500, can_purchase: true, requires_video: false };
    }
    if (path !== "/v1/coaching/requests/review-1") throw new Error(`Unexpected request: ${path}`);
    return {
      id: "review-1",
      status: "returned",
      tier: "coach",
      event: "100m",
      focus: [],
      question: "スタートの姿勢について",
      created_at: new Date().toISOString(),
      response: {
        hidden: false,
        good_point: "腕振り",
        fix_points: ["接地位置"],
        drill: "もも上げ",
        created_at: new Date().toISOString(),
        helpful_asked: false,
      },
    };
  });
  try {
    await act(async () => root.render(<ReviewRequestPage />));
    expect(container.textContent).toContain("腕振り");
    expect(container.querySelector('textarea[aria-label="追加質問の質問"]')).not.toBeNull();
    expect(container.textContent).toContain("質問する（500円）");
    expect(container.querySelectorAll("textarea")).toHaveLength(1);
    expect(container.textContent).not.toContain("確認質問を送る（無料）");
  } finally {
    await act(async () => root.unmount());
    container.remove();
    vi.unstubAllGlobals();
  }
});
