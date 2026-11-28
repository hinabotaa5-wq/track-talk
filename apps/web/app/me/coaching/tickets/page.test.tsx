/** @vitest-environment jsdom */
import { act, type ReactNode } from "react";
import { createRoot, type Root } from "react-dom/client";
import { afterEach, beforeEach, expect, it, vi } from "vitest";
import type { TicketSummary } from "@/lib/coaching-tickets";
import Page from "./page";

// チケットの販売は終了した（本体 #1031）。この画面は発行済みのチケットと ¥500 クーポンの保有状況を見るだけで、購入の導線を置かない。
const mocks = vi.hoisted(() => ({ api: vi.fn(), router: { replace: vi.fn() }, query: "" }));
vi.mock("next/navigation", () => ({ useRouter: () => mocks.router, useSearchParams: () => new URLSearchParams(mocks.query) }));
vi.mock("next/link", () => ({ default: ({ href, children }: { href: string; children: ReactNode }) => <a href={href}>{children}</a> }));
vi.mock("@/components/auth-provider", () => ({ useAuth: () => ({ token: "test", loading: false, session: { user: { id: "athlete" } } }) }));
vi.mock("@/lib/api", async (original) => ({ ...await original<typeof import("@/lib/api")>(), apiFetch: (...args: unknown[]) => mocks.api(...args) }));
const initial: TicketSummary = {
  balance: { available: 0, reserved: 0 }, products: [],
  purchases: [], ledger: [], can_purchase: false, sales_open: false,
  purchase_block_reason: "チケットの販売は終了しました。発行済みのチケットは有効期限まで利用できます。スタンダード・追加質問は申込み時にカードでお支払いください",
  unit_yen: 1000, single_price_yen: 1000, review_units: 1, validity_months: 6, is_minor: false, next_expiry: null,
  coupons: { available: 0, next_expires_at: null, amount_yen: 500, terms: "" },
};
let container: HTMLDivElement;
let root: Root;
beforeEach(() => {
  (globalThis as typeof globalThis & { IS_REACT_ACT_ENVIRONMENT: boolean }).IS_REACT_ACT_ENVIRONMENT = true;
  mocks.api.mockReset(); mocks.router.replace.mockReset(); mocks.query = ""; sessionStorage.clear();
  container = document.createElement("div"); document.body.appendChild(container); root = createRoot(container);
});
afterEach(async () => { await act(async () => root.unmount()); container.remove(); });
async function render() { await act(async () => root.render(<Page />)); }
const buttons = () => [...container.querySelectorAll("button")].map((b) => b.textContent ?? "");
const summaryResponse = (summary: TicketSummary) => (path: string) => Promise.resolve(path === "/v1/me/coaching/tickets" ? summary : { referral: null });

it("販売終了後は購入の入口を出さず、販売終了と発行済みチケットが使えることを書く（本体 #1031 S3）", async () => {
  mocks.api.mockImplementation(summaryResponse(initial));
  await render();
  expect(container.querySelector("h1")?.textContent).toBe("チケット・クーポン");
  expect(container.textContent).toContain("チケットの販売は終了しました。発行済みのチケットは有効期限まで利用できます。");
  expect(buttons().some((b) => b.includes("購入"))).toBe(false);
  expect(container.textContent).not.toContain("5,000");
  expect(buttons()).toContain("チケット情報を再読み込み");
  expect(mocks.api.mock.calls.some(([, options]) => options?.method === "POST")).toBe(false);
});

it("発行済みのチケットの残りの枚数と有効期限は出し続ける（本体 #1031 S4）", async () => {
  mocks.api.mockImplementation(summaryResponse({
    ...initial, balance: { available: 3, reserved: 0 },
    next_expiry: { expires_at: "2027-05-01T00:00:00Z", available: 3 },
  }));
  await render();
  expect(container.textContent).toContain("チケットの残り 3枚");
  expect(container.textContent).toContain("いちばん早い有効期限");
});

it("¥500 クーポンの枚数といちばん早い有効期限を出し、使う入口へ案内する（本体 #1031）", async () => {
  mocks.api.mockImplementation(summaryResponse({ ...initial, coupons: { available: 2, next_expires_at: "2027-05-14T09:00:00Z", amount_yen: 500, terms: "" } }));
  await render();
  const section = container.querySelector('section[aria-label="クーポンの残り"]');
  expect(section?.textContent).toContain("¥500クーポンの残り 2枚");
  expect(section?.textContent).toContain("いちばん早い有効期限：");
  expect(section?.textContent).toContain("申込みで1回に1枚使えます。チケットとは一緒に使えません。");
  expect(section?.querySelector('a[href="/coaching"]')?.textContent).toBe("コーチを選んでクーポンを使う");
});

it("クーポンが0枚なら有効期限は出さず、紹介の説明ページへ案内する", async () => {
  mocks.api.mockImplementation(summaryResponse(initial));
  await render();
  const section = container.querySelector('section[aria-label="クーポンの残り"]');
  expect(section?.textContent).toContain("¥500クーポンの残り 0枚");
  expect(section?.textContent).not.toContain("いちばん早い有効期限");
  expect(section?.querySelector('a[href="/referral"]')?.textContent).toBe("紹介でクーポンをもらう方法");
});
