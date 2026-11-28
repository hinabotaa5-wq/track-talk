/** @vitest-environment jsdom */
import { act } from "react";
import { createRoot, type Root } from "react-dom/client";
import { afterEach, beforeEach, expect, it, vi } from "vitest";
import type { TicketSummary } from "@/lib/coaching-tickets";
import type { CoachingMe } from "@/lib/types";
import Page from "./page";

// マイコーチング画面（本体 #1108）：取得した内容は利用者と組で持ち、切り替え・履歴復帰で前の利用者の分を出さない（#1011 の方式）。
const auth = vi.hoisted(() => ({ token: "tok-a" as string | null, session: { user: { id: "user-a" } } as { user: { id: string } } | null, loading: false }));
const api = vi.hoisted(() => vi.fn());
vi.mock("@/components/auth-provider", () => ({ useAuth: () => auth }));
vi.mock("@/lib/api", async (original) => ({ ...await original<typeof import("@/lib/api")>(), apiFetch: (...args: unknown[]) => api(...args) }));

const me: CoachingMe = { birth_month: "1995-04", is_minor: false, guardian: null, push_devices: 0, demo: false };
const summary = (available: number): TicketSummary => ({
  balance: { available, reserved: 0 }, products: [], purchases: [], ledger: [], can_purchase: false, sales_open: false,
  purchase_block_reason: "", unit_yen: 1000, single_price_yen: 1000, review_units: 1, validity_months: 6, is_minor: false, next_expiry: null,
  coupons: { available: 0, next_expires_at: null, amount_yen: 500, terms: "" },
});
const respond = (tickets: TicketSummary) => (path: string) => Promise.resolve(path === "/v1/me/coaching/tickets" ? tickets : me);

let container: HTMLDivElement;
let root: Root;
beforeEach(() => {
  (globalThis as typeof globalThis & { IS_REACT_ACT_ENVIRONMENT: boolean }).IS_REACT_ACT_ENVIRONMENT = true;
  auth.token = "tok-a"; auth.session = { user: { id: "user-a" } };
  api.mockReset();
  container = document.createElement("div"); document.body.appendChild(container); root = createRoot(container);
});
afterEach(async () => { await act(async () => root.unmount()); container.remove(); });

async function render() {
  await act(async () => root.render(<Page />));
  await act(async () => { await Promise.resolve(); });
}

it("利用者を切り替えると、前の利用者の残高・準備状況は新しい取得が終わるまで出さない（本体 #1108 T1）", async () => {
  api.mockImplementation(respond(summary(3)));
  await render();
  expect(container.textContent).toContain("チケットの残り 3枚");
  expect(container.textContent).toContain("継続プランの準備");
  let resolveMe!: (value: CoachingMe) => void;
  let resolveTickets!: (value: TicketSummary) => void;
  api.mockImplementation((path: string) => (path === "/v1/me/coaching/tickets"
    ? new Promise<TicketSummary>((res) => { resolveTickets = res; })
    : new Promise<CoachingMe>((res) => { resolveMe = res; })));
  auth.token = "tok-b"; auth.session = { user: { id: "user-b" } };
  await render();
  expect(container.textContent).not.toContain("チケットの残り 3枚");
  expect(container.textContent).not.toContain("継続プランの準備");
  await act(async () => { resolveMe(me); resolveTickets(summary(5)); });
  await act(async () => { await Promise.resolve(); });
  expect(container.textContent).toContain("チケットの残り 5枚");
  expect(container.textContent).toContain("継続プランの準備");
});

it("履歴キャッシュから戻ったら（pageshow persisted）残高と準備状況を取り直す（本体 #1108 T2）", async () => {
  api.mockImplementation(respond(summary(3)));
  await render();
  expect(container.textContent).toContain("チケットの残り 3枚");
  api.mockClear();
  api.mockImplementation(respond(summary(4)));
  const restored = new Event("pageshow");
  Object.defineProperty(restored, "persisted", { value: true });
  await act(async () => { window.dispatchEvent(restored); });
  await act(async () => { await Promise.resolve(); });
  const paths = api.mock.calls.map(([path]) => path);
  expect(paths).toContain("/v1/me/coaching/tickets");
  expect(paths).toContain("/v1/me/coaching");
  expect(container.textContent).toContain("チケットの残り 4枚");
});
