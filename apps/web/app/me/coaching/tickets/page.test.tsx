/** @vitest-environment jsdom */
import { act, type ReactNode } from "react";
import { createRoot, type Root } from "react-dom/client";
import { afterEach, beforeEach, expect, it, vi } from "vitest";
import { ApiRequestError } from "@/lib/api";
import type { TicketSummary } from "@/lib/coaching-tickets";
import Page from "./page";

const mocks = vi.hoisted(() => ({ api: vi.fn(), router: { replace: vi.fn() }, query: "" }));
vi.mock("next/navigation", () => ({ useRouter: () => mocks.router, useSearchParams: () => new URLSearchParams(mocks.query) }));
vi.mock("next/link", () => ({ default: ({ href, children }: { href: string; children: ReactNode }) => <a href={href}>{children}</a> }));
vi.mock("@/components/auth-provider", () => ({ useAuth: () => ({ token: "test", loading: false, session: { user: { id: "athlete" } } }) }));
vi.mock("@/lib/api", async (original) => ({ ...await original<typeof import("@/lib/api")>(), apiFetch: (...args: unknown[]) => mocks.api(...args) }));
const initial: TicketSummary = {
  balance: { available: 0, reserved: 0 }, products: [{ id: "single", title: "チケット1枚", quantity: 1, amount_yen: 500 }],
  purchases: [], ledger: [], can_purchase: true, purchase_block_reason: "", unit_yen: 500,
};
let container: HTMLDivElement;
let root: Root;
beforeEach(() => {
  (globalThis as typeof globalThis & { IS_REACT_ACT_ENVIRONMENT: boolean }).IS_REACT_ACT_ENVIRONMENT = true;
  mocks.api.mockReset(); mocks.router.replace.mockReset(); mocks.query = ""; sessionStorage.clear();
  container = document.createElement("div"); document.body.appendChild(container); root = createRoot(container);
});
afterEach(async () => { await act(async () => root.unmount()); container.remove(); vi.useRealTimers(); });
async function render() { await act(async () => root.render(<Page />)); }
async function buy() {
  const button = [...container.querySelectorAll("button")].find((b) => b.textContent === "購入する");
  if (!button) throw new Error("購入ボタンがありません");
  await act(async () => button.click());
}

it("決済から戻っただけでは残高を増やさず、サーバー確定後に元の相談画面へ戻る", async () => {
  vi.useFakeTimers(); mocks.query = "purchase=payment-id";
  let paid = false;
  mocks.api.mockImplementation(async (path: string) => path.includes("/purchases/")
    ? { status: paid ? "paid" : "pending", return_path: "/coaching/new" }
    : { ...initial, balance: { available: paid ? 5 : 0, reserved: 0 } });
  await render();
  expect(container.textContent).toContain("利用できるチケット：0枚");
  expect(container.textContent).toContain("お支払いの確認中");
  expect(mocks.router.replace).not.toHaveBeenCalled();
  paid = true;
  await act(async () => vi.advanceTimersByTimeAsync(2000));
  expect(container.textContent).toContain("利用できるチケット：5枚");
  expect(mocks.router.replace).toHaveBeenCalledWith("/coaching/new");
});

it("1枚商品には1枚あたりの金額を出さず、まとめ買いには1枚あたり576円を出す", async () => {
  mocks.api.mockResolvedValue({
    ...initial,
    products: [
      { id: "single", title: "チケット1枚", quantity: 1, amount_yen: 600 },
      { id: "five", title: "チケット5枚", quantity: 5, amount_yen: 2880 },
    ],
  });
  await render();
  expect(container.textContent).toContain("600円");
  expect(container.textContent).not.toContain("1枚あたり600円");
  expect(container.textContent).toContain("1枚あたり576円");
});

it("購入通信の再試行では同じ操作IDを使い、二重購入を防ぐ", async () => {
  let requests = 0;
  mocks.api.mockImplementation(async (_path: string, options?: { method?: string }) => {
    if (options?.method === "POST") {
      if (++requests === 1) throw new ApiRequestError(502, { error: { code: "payment_unavailable", message: "再試行してください" } });
      return { paid: true, return_path: "/coaching/new" };
    }
    return initial;
  });
  await render(); await buy(); await buy();
  const bodies = mocks.api.mock.calls.filter(([, options]) => options?.method === "POST").map(([, options]) => JSON.parse(options.body));
  expect(bodies).toHaveLength(2);
  expect(bodies[0].request_key).toBe(bodies[1].request_key);
  expect(bodies[0].product_id).toBe("single");
});

it("利用設定の案内を表示せず、購入できない場合は購入を止める", async () => {
  mocks.api.mockResolvedValue({ ...initial, can_purchase: false, purchase_block_reason: "保護者の同意が必要です" });
  await render();
  expect(container.textContent).not.toContain("利用設定");
  await buy();
  expect(mocks.api.mock.calls.some(([, options]) => options?.method === "POST")).toBe(false);
});

it("販売を止めている間は商品と購入ボタンを出さず、終了の案内と残高だけを出す", async () => {
  mocks.api.mockResolvedValue({
    ...initial, products: [], can_purchase: false, sales_open: false, balance: { available: 3, reserved: 0 },
    purchase_block_reason: "チケットの新規販売は終了しました。動画添削・追加相談は申込み時にカードまたはPayPayでお支払いください",
  });
  await render();
  expect(container.textContent).toContain("新規販売は終了しました");
  expect(container.textContent).toContain("利用できるチケット：3枚");
  expect([...container.querySelectorAll("button")].some((b) => b.textContent === "購入する")).toBe(false);
});
