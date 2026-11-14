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

it("承認済みコーチの評価・除外の注記・順番に戻すボタンを表示しない", async () => {
  mocks.api.mockImplementation(async (path: string) => {
    if (path.startsWith("/v1/admin/coach-applications?status=approved")) {
      return {
        applications: [{
          profile_id: "coach-1", nickname: "コーチA", status: "approved", tier: "coach",
          events: ["100m"], applied_at: "2026-11-14T00:00:00Z",
          rating_count: 5, rating_average: 2.5, rating_paused: true,
        }],
        meta: { total: 1 },
      };
    }
    return emptyList("reports");
  });
  await act(async () => root.render(<ConfirmProvider><Page /></ConfirmProvider>));
  const coachesTab = [...container.querySelectorAll('[role="tab"]')].find((tab) => tab.textContent === "コーチ");
  await act(async () => coachesTab?.dispatchEvent(new MouseEvent("click", { bubbles: true })));
  const approvedButton = [...container.querySelectorAll("button")].find((button) => button.textContent === "承認済み");
  await act(async () => approvedButton?.click());

  expect(container.textContent).toContain("コーチA");
  expect(container.textContent).toContain("コーチを止める");
  expect(container.textContent).not.toContain("評価:");
  expect(container.textContent).not.toContain("★2.5（5件）");
  expect(container.textContent).not.toContain("評価が低いため、依頼が回っていません。");
  expect(container.textContent).not.toContain("順番に戻す");
});
