/** @vitest-environment jsdom */
import { act } from "react";
import { createRoot, type Root } from "react-dom/client";
import { afterEach, beforeEach, expect, it, vi } from "vitest";
import type { AdminCoachPriceApplication } from "@/lib/coach-price";
import Page from "./page";

// 価格申請の管理画面（本体 #1108）：一覧は利用者と組で持ち、切り替え・履歴復帰で前の利用者の分を出さない（#1011 の方式）。
const auth = vi.hoisted(() => ({ token: "tok-a" as string | null, session: { user: { id: "admin-a" } } as { user: { id: string } } | null }));
const api = vi.hoisted(() => vi.fn());
vi.mock("@/components/auth-provider", () => ({ useAuth: () => auth }));
vi.mock("@/lib/api", async (original) => ({ ...await original<typeof import("@/lib/api")>(), apiFetch: (...args: unknown[]) => api(...args) }));

function application(coachName: string): AdminCoachPriceApplication {
  return {
    id: `app-${coachName}`, coach_id: "coach-1", coach_name: coachName, coach_status: "approved", coach_country_code: "JP",
    plan: "standard", price_yen: 2000, current_price_yen: 1000, source: "price", status: "pending",
    created_at: "2026-11-26T00:00:00Z", updated_at: "2026-11-26T00:00:00Z", approved_at: null, approved_by_nickname: null, applied_at: null,
  };
}
const respond = (apps: AdminCoachPriceApplication[]) => (path: string) =>
  Promise.resolve(path.includes("coach-prices") ? { applications: apps } : { applications: [] });

let container: HTMLDivElement;
let root: Root;
beforeEach(() => {
  (globalThis as typeof globalThis & { IS_REACT_ACT_ENVIRONMENT: boolean }).IS_REACT_ACT_ENVIRONMENT = true;
  auth.token = "tok-a"; auth.session = { user: { id: "admin-a" } };
  api.mockReset();
  container = document.createElement("div"); document.body.appendChild(container); root = createRoot(container);
});
afterEach(async () => { await act(async () => root.unmount()); container.remove(); });

async function render() {
  await act(async () => root.render(<Page />));
  await act(async () => { await Promise.resolve(); });
}

it("利用者を切り替えると、前の利用者の申請一覧は新しい取得が終わるまで出さない（本体 #1108 T1）", async () => {
  api.mockImplementation(respond([application("コーチ甲")]));
  await render();
  expect(container.textContent).toContain("コーチ甲");
  let resolveNext!: (value: { applications: AdminCoachPriceApplication[] }) => void;
  const next = new Promise<{ applications: AdminCoachPriceApplication[] }>((res) => { resolveNext = res; });
  api.mockImplementation((path: string) => (path.includes("coach-prices") ? next : Promise.resolve({ applications: [] })));
  auth.token = "tok-b"; auth.session = { user: { id: "admin-b" } };
  await render();
  expect(container.textContent).not.toContain("コーチ甲");
  await act(async () => { resolveNext({ applications: [application("コーチ乙")] }); });
  await act(async () => { await Promise.resolve(); });
  expect(container.textContent).toContain("コーチ乙");
});

it("履歴キャッシュから戻ったら（pageshow persisted）申請一覧を取り直す（本体 #1108 T2）", async () => {
  api.mockImplementation(respond([application("コーチ甲")]));
  await render();
  expect(container.textContent).toContain("コーチ甲");
  api.mockClear();
  api.mockImplementation(respond([application("コーチ乙")]));
  const restored = new Event("pageshow");
  Object.defineProperty(restored, "persisted", { value: true });
  await act(async () => { window.dispatchEvent(restored); });
  await act(async () => { await Promise.resolve(); });
  expect(api.mock.calls.some(([path]) => String(path).includes("coach-prices"))).toBe(true);
  expect(container.textContent).toContain("コーチ乙");
});
