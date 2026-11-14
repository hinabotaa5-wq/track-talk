/** @vitest-environment jsdom */
import { act, type ReactNode } from "react";
import { createRoot, type Root } from "react-dom/client";
import { afterEach, beforeEach, expect, it, vi } from "vitest";
import { ConfirmProvider } from "@/components/confirm-dialog";
import Page from "./page";

const mocks = vi.hoisted(() => ({ api: vi.fn(), replace: vi.fn() }));
vi.mock("next/link", () => ({ default: ({ href, children }: { href: string; children: ReactNode }) => <a href={href}>{children}</a> }));
vi.mock("next/navigation", () => ({ useRouter: () => ({ replace: mocks.replace }) }));
vi.mock("@/components/auth-provider", () => ({
  useAuth: () => ({
    session: { access_token: "admin-token" },
    profile: { is_admin: true },
    token: "admin-token",
    loading: false,
  }),
}));
vi.mock("@/lib/api", async (original) => ({ ...await original<typeof import("@/lib/api")>(), apiFetch: (...args: unknown[]) => mocks.api(...args) }));

function emptyList(key: string) {
  return { [key]: [], meta: { page: 1, per_page: 20, total: 0 } };
}

function calledPaths() {
  return mocks.api.mock.calls.map((call) => String(call[0]));
}

let container: HTMLDivElement;
let root: Root;
beforeEach(() => {
  (globalThis as typeof globalThis & { IS_REACT_ACT_ENVIRONMENT: boolean }).IS_REACT_ACT_ENVIRONMENT = true;
  mocks.api.mockReset();
  mocks.replace.mockReset();
  mocks.api.mockImplementation((path: string) =>
    Promise.resolve(path.startsWith("/v1/admin/reports") ? emptyList("reports") : emptyList("inquiries")),
  );
  container = document.createElement("div");
  document.body.appendChild(container);
  root = createRoot(container);
});
afterEach(async () => {
  await act(async () => root.unmount());
  container.remove();
});

it("初期表示では開いている通報タブだけを読む", async () => {
  await act(async () => {
    root.render(<Page />);
  });
  await act(async () => {});
  expect(calledPaths()).toContainEqual(expect.stringContaining("/v1/admin/reports"));
  expect(calledPaths().filter((p) => p.includes("/v1/admin/inquiries"))).toEqual([]);
});

it("コーチタブでは評価の平均・件数・停止表示と「順番に戻す」が出ない", async () => {
  const approved = {
    profile_id: "coach-1", status: "approved", nickname: "テストコーチ",
    service_types: ["video_review"], tier: "coach", events: ["100m"],
    best_record: "", achievements: "", years: 3, sns_urls: [], hitokoto: "",
    applied_at: "2026-11-01T00:00:00Z", reviewed_at: null, on_duty: true, daily_cap: 5,
    photo_url: "", pledged: true, payouts_ready: true,
    stripe_linked: true, payouts_enabled: true,
    rating_count: 8, rating_average: 2.1, rating_paused: true,
  };
  mocks.api.mockImplementation((path: string) =>
    Promise.resolve(
      path.startsWith("/v1/admin/coach-applications")
        ? { applications: path.includes("status=approved") ? [approved] : [], meta: { page: 1, per_page: 50, total: 1 } }
        : path.startsWith("/v1/admin/reports") ? emptyList("reports") : emptyList("inquiries"),
    ),
  );
  await act(async () => {
    root.render(<ConfirmProvider><Page /></ConfirmProvider>);
  });
  await act(async () => {});
  const coachTab = Array.from(container.querySelectorAll('[role="tab"]')).find((b) =>
    b.textContent === "コーチ",
  )!;
  await act(async () => {
    coachTab.dispatchEvent(new MouseEvent("click", { bubbles: true }));
  });
  await act(async () => {});
  const approvedFilter = Array.from(container.querySelectorAll("button")).find((b) =>
    b.textContent?.includes("承認済み"),
  )!;
  await act(async () => {
    approvedFilter.dispatchEvent(new MouseEvent("click", { bubbles: true }));
  });
  await act(async () => {});
  expect(container.textContent).toContain("テストコーチ");
  expect(container.textContent).not.toContain("★8");
  expect(container.textContent).not.toContain("評価:");
  expect(container.textContent).not.toContain("順番に戻す");
  expect(container.textContent).not.toContain("依頼が回っていません");
});

it("隠れていたタブは開いたときに読む", async () => {
  await act(async () => {
    root.render(<Page />);
  });
  await act(async () => {});
  const before = calledPaths().filter((p) => p.includes("source=feedback")).length;
  expect(before).toBe(0);
  const feedbackTab = Array.from(container.querySelectorAll('[role="tab"]')).find((b) =>
    b.textContent?.includes("なんでも書いてね"),
  )!;
  await act(async () => {
    feedbackTab.dispatchEvent(new MouseEvent("click", { bubbles: true }));
  });
  await act(async () => {});
  expect(calledPaths().filter((p) => p.includes("source=feedback"))).not.toEqual([]);
});
