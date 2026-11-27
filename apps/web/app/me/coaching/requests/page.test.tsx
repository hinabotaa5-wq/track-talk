/** @vitest-environment jsdom */
import { act, type ReactNode } from "react";
import { createRoot, type Root } from "react-dom/client";
import { afterEach, beforeEach, expect, it, vi } from "vitest";
import type { MyReviewRequestListItem } from "@/lib/types";
import Page from "./page";

const mocks = vi.hoisted(() => ({ api: vi.fn(), router: { replace: vi.fn() } }));
vi.mock("next/navigation", () => ({ useRouter: () => mocks.router }));
vi.mock("next/link", () => ({ default: ({ href, children, className }: { href: string; children: ReactNode; className?: string }) => <a href={href} className={className}>{children}</a> }));
vi.mock("@/components/auth-provider", () => ({ useAuth: () => ({ token: "test", loading: false, session: { user: { id: "athlete" } } }) }));
vi.mock("@/lib/api", async (original) => ({ ...await original<typeof import("@/lib/api")>(), apiFetch: (...args: unknown[]) => mocks.api(...args) }));

// 一覧の応答は6項目＋plan だけ（本体 #1072・#1053）。
const oneRequest: MyReviewRequestListItem = {
  id: "req-1",
  plan: "standard",
  event: "100m",
  status: "in_review",
  canceled_at: null,
  withdrawal: null,
  created_at: "2026-11-17T00:00:00Z",
};

let container: HTMLDivElement;
let root: Root;
beforeEach(() => {
  (globalThis as typeof globalThis & { IS_REACT_ACT_ENVIRONMENT: boolean }).IS_REACT_ACT_ENVIRONMENT = true;
  mocks.api.mockReset();
  mocks.router.replace.mockReset();
  container = document.createElement("div");
  document.body.appendChild(container);
  root = createRoot(container);
});
afterEach(async () => { await act(async () => root.unmount()); container.remove(); });
async function render() { await act(async () => root.render(<Page />)); }

it("依頼0件では案内文とコーチ一覧リンクを出し、新しく頼むと /coaching/new リンクは出さない", async () => {
  mocks.api.mockResolvedValue({ requests: [], meta: {} });
  await render();
  expect(container.querySelector("h1")?.textContent).toBe("スタンダードの依頼");
  expect(container.textContent).toContain("まだ依頼はありません。");
  expect(container.textContent).toContain("から依頼できます。");
  const coachList = container.querySelector('a[href="/coaching"]');
  expect(coachList?.textContent).toBe("コーチ一覧");
  expect(coachList?.className).toContain("text-blue-300");
  expect(coachList?.className).toContain("underline");
  expect(container.textContent).not.toContain("新しく頼む");
  expect(container.querySelector('a[href="/coaching/new"]')).toBeNull();
});

it("依頼が1件あるときは案内文も新しく頼むも出さず、依頼行は詳細へリンクする", async () => {
  mocks.api.mockResolvedValue({ requests: [oneRequest], meta: {} });
  await render();
  expect(container.querySelector("h1")?.textContent).toBe("スタンダードの依頼");
  expect(container.textContent).not.toContain("まだ依頼はありません。");
  expect(container.textContent).not.toContain("から依頼できます。");
  expect(container.textContent).not.toContain("新しく頼む");
  expect(container.querySelector('a[href="/coaching/new"]')).toBeNull();
  expect(container.querySelector('a[href="/coaching"]')).toBeNull();
  expect(container.querySelector('a[href="/coaching/requests/req-1"]')).not.toBeNull();
});

it("6項目だけの応答で、種目・状態ラベル・キャンセル・コーチ返却を出し分ける", async () => {
  mocks.api.mockResolvedValue({
    requests: [
      oneRequest,
      { ...oneRequest, id: "req-2", status: "canceled", canceled_at: "2026-11-17T01:00:00Z" },
      { ...oneRequest, id: "req-3", status: "queued", withdrawal: { withdrawn_at: "2026-11-17T02:00:00Z", from_status: "in_review", reason: "", reason_code: null } },
    ],
    meta: {},
  });
  await render();
  expect(mocks.api.mock.calls[0][0]).toBe("/v1/me/coaching/requests?per_page=50");
  const rows = [...container.querySelectorAll("li")];
  expect(rows.map((li) => li.querySelector("a")?.getAttribute("href"))).toEqual(["/coaching/requests/req-1", "/coaching/requests/req-2", "/coaching/requests/req-3"]);
  expect(rows[0].textContent).toContain("100m");
  expect(rows[1].textContent).toContain("依頼をキャンセルしました");
  expect(rows[2].textContent).toContain("コーチが依頼を返却しました");
  expect(rows[0].textContent).not.toContain("依頼をキャンセルしました");
  expect(rows[0].textContent).not.toContain("コーチが依頼を返却しました");
});
