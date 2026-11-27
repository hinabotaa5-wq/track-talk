/** @vitest-environment jsdom */
import { act } from "react";
import { createRoot, type Root } from "react-dom/client";
import { afterEach, beforeEach, expect, it, vi } from "vitest";
import type { AdminCoachOfferApplication } from "@/lib/coach-offer";

// 管理画面の添削項目・見本動画の申請（本体 #1086 W2・W8）。
const apiFetch = vi.fn();
vi.mock("@/lib/api", async (original) => ({ ...await original<typeof import("@/lib/api")>(), apiFetch: (...args: unknown[]) => apiFetch(...args) }));
const { AdminCoachOffers } = await import("./admin-coach-offers");

function row(status: AdminCoachOfferApplication["status"], extra: Partial<AdminCoachOfferApplication> = {}): AdminCoachOfferApplication {
  return {
    id: `app-${status}`, coach_id: "coach-1", coach_name: "テストコーチ", coach_status: "approved",
    items: ["スタート局面の詳しい分析"], demo_videos: [{ url: "https://example.com/after.mp4" }],
    current_items: ["接地の分析"], current_demo_videos: [{ url: "https://example.com/before.mp4" }],
    status, created_at: "2026-11-27T00:00:00Z", updated_at: "2026-11-27T00:00:00Z",
    approved_at: null, approved_by_nickname: null, applied_at: null, ...extra,
  };
}

let root: Root;
let container: HTMLDivElement;
beforeEach(() => {
  (globalThis as typeof globalThis & { IS_REACT_ACT_ENVIRONMENT: boolean }).IS_REACT_ACT_ENVIRONMENT = true;
  container = document.createElement("div"); document.body.appendChild(container); root = createRoot(container);
  apiFetch.mockReset();
});
afterEach(async () => { await act(async () => root.unmount()); container.remove(); });

it("W8: 承認待ちの申請は、変更前→変更後のタイトル一覧と見本動画の再生を出す", async () => {
  apiFetch.mockResolvedValue({ applications: [row("pending")] });
  await act(async () => root.render(<AdminCoachOffers token="admin" />));
  const li = container.querySelector('[data-testid="offer-application-app-pending"]')!;
  const before = li.querySelector('[data-testid="offer-before"]')!;
  const after = li.querySelector('[data-testid="offer-after"]')!;
  expect(before.textContent).toContain("変更前（いま有効な内容）");
  expect(before.textContent).toContain("接地の分析");
  expect(before.querySelector("video")?.getAttribute("src")).toBe("https://example.com/before.mp4");
  expect(after.textContent).toContain("変更後（申請の内容）");
  expect(after.textContent).toContain("スタート局面の詳しい分析");
  expect(after.querySelector("video")?.getAttribute("src")).toBe("https://example.com/after.mp4");
  expect(after.querySelector("video")?.hasAttribute("controls")).toBe(true);
  expect(before.compareDocumentPosition(after) & Node.DOCUMENT_POSITION_FOLLOWING).toBeTruthy();
});

it("W2: 承認すると API に承認を送り、承認者・日時が出る", async () => {
  apiFetch.mockResolvedValueOnce({ applications: [row("pending")] });
  await act(async () => root.render(<AdminCoachOffers token="admin" />));
  apiFetch.mockResolvedValueOnce({ application: {} });
  apiFetch.mockResolvedValueOnce({ applications: [row("applied", { approved_at: "2026-11-27T01:00:00Z", approved_by_nickname: "運営A", applied_at: "2026-11-27T01:00:00Z" })] });
  const approve = [...container.querySelectorAll("button")].find((b) => b.textContent === "承認する")!;
  await act(async () => approve.click());
  expect(apiFetch).toHaveBeenCalledWith("/v1/admin/coaching/coach-offers/app-pending/approve", { method: "POST", token: "admin" });
  expect(container.textContent).toContain("運営A");
  expect(container.textContent).toContain("有効");
  expect(container.querySelector("button")).toBeNull();
});

it("見本も追加項目もない申請は「なし」と出す（全削除の申請）", async () => {
  apiFetch.mockResolvedValue({ applications: [row("pending", { items: [], demo_videos: [] })] });
  await act(async () => root.render(<AdminCoachOffers token="admin" />));
  const after = container.querySelector('[data-testid="offer-after"]')!;
  expect(after.textContent).toContain("追加項目なし");
  expect(after.textContent).toContain("見本動画なし");
});
