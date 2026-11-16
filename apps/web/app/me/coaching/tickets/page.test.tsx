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
  balance: { available: 0, reserved: 0 }, products: [{ id: "six", title: "チケット6枚", quantity: 6, amount_yen: 2500 }],
  purchases: [], ledger: [], can_purchase: true, purchase_block_reason: "", unit_yen: 500,
  single_price_yen: 500, review_units: 1, validity_months: 6, is_minor: false, next_expiry: null,
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
// 購入は「購入内容を確認する」→ 最終確認の「確定してお支払いへ」の2段（本体 #569）。
const reviewButton = () => [...container.querySelectorAll("button")].find((b) => b.textContent === "購入内容を確認する");
async function buy() {
  const confirmed = () => [...container.querySelectorAll("button")].find((b) => b.textContent?.startsWith("確定してお支払いへ"));
  if (!confirmed()) {
    const button = reviewButton();
    if (!button) throw new Error("購入ボタンがありません");
    await act(async () => button.click());
  }
  const next = confirmed();
  if (!next) return;
  await act(async () => next.click());
}

it("決済確認中は購入を止め、確定後に紹介IDと元の画面へのリンクを表示する", async () => {
  vi.useFakeTimers(); mocks.query = "purchase=payment-id";
  let paid = false;
  mocks.api.mockImplementation(async (path: string) => path.endsWith("/referral")
    ? { referral: { code: "K7M3QX", active: true, friend_count: 0, granted_at: null, expires_at: null } }
    : path.includes("/purchases/")
    ? { status: paid ? "paid" : "pending", return_path: "/coaching/new" }
    : { ...initial, balance: { available: paid ? 6 : 0, reserved: 0 } });
  await render();
  expect(reviewButton()?.disabled).toBe(true);
  expect(container.textContent).toContain("お支払いの確認中");
  expect(mocks.router.replace).not.toHaveBeenCalled();
  paid = true;
  await act(async () => vi.advanceTimersByTimeAsync(2000));
  expect(container.textContent).toContain("購入内容を反映しました");
  expect(container.textContent).toContain("K7M3QX");
  expect(container.querySelector('a[href="/coaching/new"]')?.textContent).toBe("元の画面に戻る");
});

// 本体 #691：コーチ詳細から来た購入は、その戻り先を決済 API に渡し、確定後はそのコーチ詳細へのリンクを出す。
it("コーチ詳細から来た購入は戻り先を API に渡し、確定後にそのコーチ詳細へ戻るリンクを出す", async () => {
  const coachPath = "/coaches/12345678-1234-1234-1234-123456789abc";
  mocks.query = `return_to=${encodeURIComponent(coachPath)}`;
  mocks.api.mockImplementation(async (path: string, options?: { method?: string }) => options?.method === "POST"
    ? { paid: true, return_path: coachPath }
    : path.endsWith("/referral") ? { referral: null } : initial);
  await render(); await buy();
  await act(async () => container.querySelector<HTMLInputElement>('input[type="checkbox"]')?.click());
  await buy();
  const body = JSON.parse(mocks.api.mock.calls.find(([, options]) => options?.method === "POST")![1].body);
  expect(body.return_path).toBe(coachPath);
  expect(container.textContent).toContain("購入内容を反映しました");
  expect(container.querySelector(`a[href="${coachPath}"]`)?.textContent).toBe("元の画面に戻る");
});

it("不正な戻り先は API に渡さず、購入画面を戻り先にする", async () => {
  mocks.query = `return_to=${encodeURIComponent("/coaches/12345678-1234-1234-1234-123456789abc/../../admin")}`;
  mocks.api.mockImplementation(async (_path: string, options?: { method?: string }) => options?.method === "POST" ? { checkout_url: "" } : initial);
  await render(); await buy();
  await act(async () => container.querySelector<HTMLInputElement>('input[type="checkbox"]')?.click());
  await buy();
  const body = JSON.parse(mocks.api.mock.calls.find(([, options]) => options?.method === "POST")![1].body);
  expect(body.return_path).toBe("/me/coaching/tickets");
});

it("6枚2,500円の購入カードは商品名・価格・1行・ボタンだけで、条件の説明文を置かない", async () => {
  mocks.api.mockResolvedValue(initial);
  await render();
  const card = reviewButton()?.parentElement;
  expect(card?.textContent).toContain("チケット6枚");
  expect(card?.textContent).toContain("2,500円（税込）");
  expect(card?.textContent).toContain("1回あたり約417円・有効期限6か月");
  expect(card?.textContent).not.toContain("1回ずつ払うと");
  expect(card?.querySelector("ul, li")).toBeNull();
  for (const old of ["1枚で1回プラン1件を申し込めます", "有効期限は購入から6か月。過ぎると失効し、延長はしません", "購入した本人だけが使えます。譲渡はできません", "原則、購入者都合の払戻しはしません。"]) {
    expect(container.textContent).not.toContain(old);
  }
  expect(container.textContent).not.toContain("1回プラン1件に1枚を使います。");
  expect(card?.textContent).not.toContain("当社");
  expect(card?.querySelector("s, del")).toBeNull();
  expect(reviewButton()?.textContent).toBe("購入内容を確認する");
});

it("購入の前に最終確認を出し、確定を押すまで支払いへ進まない", async () => {
  vi.useFakeTimers({ now: new Date("2026-11-20T10:00:00+09:00") });
  mocks.api.mockResolvedValue(initial);
  await render();
  await act(async () => reviewButton()!.click());
  const confirm = container.querySelector('section[aria-labelledby="purchase-confirm-heading"]')!;
  expect(confirm.textContent).toContain("1回プランのチケット6枚（1枚で1回プラン1件）。購入した本人だけが使えます。譲ることはできません。");
  expect(confirm.textContent).toContain("2,500円（税込）。6枚分をまとめてカードで先に支払います。支払いが済むと購入が確定し、すぐ使えます。");
  expect(confirm.textContent).toContain("2027/05/20 10:00");
  // 最終確認にも例外4項目を番号付きで出す（本体 #572）。
  expect(confirm.textContent).toContain("購入者の都合による払戻しは原則として行いません");
  expect(confirm.textContent).toContain("（1）サービス終了時は");
  expect(confirm.textContent).toContain("（2）法令で認められる取消し・解除は");
  const refund = Array.from(confirm.querySelectorAll("dt")).find((label) => label.textContent === "払戻し")?.nextElementSibling;
  expect(refund?.textContent).toContain("確保したチケット1枚を戻します。これは確保の解除で、金銭の払戻しには当たりません。");
  expect(refund?.textContent).toContain("（3）運営者またはコーチの責めに帰すべき事由で1回プランを受けられなかった場合、または運営者の都合で提供が著しく遅れた場合は、チケット1枚につき500円を返金します。");
  expect(refund?.textContent).toContain("（4）運営者またはコーチの都合で有効期限内に1回プランを受けられなかったチケットは、失効後でも1枚につき500円を返金します。");
  expect(refund?.textContent).not.toMatch(/当社|チケットを戻すか返金/);
  expect(mocks.api.mock.calls.some(([, options]) => options?.method === "POST")).toBe(false);
});

it("18歳未満は、確認のチェックを入れないと購入に進めず、入れると宣言を一緒に送る", async () => {
  mocks.api.mockImplementation(async (_path: string, options?: { method?: string }) => options?.method === "POST" ? { checkout_url: "" } : { ...initial, is_minor: true });
  await render();
  await buy();
  expect(container.textContent).toContain("未成年の方は保護者に許可を取ってください");
  expect(mocks.api.mock.calls.some(([, options]) => options?.method === "POST")).toBe(false);
  const box = container.querySelector<HTMLInputElement>('input[type="checkbox"]')!;
  await act(async () => box.click());
  await buy();
  const body = JSON.parse(mocks.api.mock.calls.find(([, options]) => options?.method === "POST")![1].body);
  expect(body.guardian_permission).toBe(true);
  expect(body.guardian_permission_shown_at).toBeTruthy();
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
  await render(); await buy();
  await act(async () => container.querySelector<HTMLInputElement>('input[type="checkbox"]')!.click());
  await buy(); await buy();
  const bodies = mocks.api.mock.calls.filter(([, options]) => options?.method === "POST").map(([, options]) => JSON.parse(options.body));
  expect(bodies).toHaveLength(2);
  expect(bodies[0].request_key).toBe(bodies[1].request_key);
  expect(bodies[0].product_id).toBe("six");
});

it("利用設定の案内を表示せず、購入できない場合は購入を止める", async () => {
  mocks.api.mockResolvedValue({ ...initial, can_purchase: false, purchase_block_reason: "保護者の同意が必要です" });
  await render();
  expect(container.textContent).not.toContain("利用設定");
  await buy();
  expect(mocks.api.mock.calls.some(([, options]) => options?.method === "POST")).toBe(false);
});

it("販売を止めている間も終了を案内し、残りの枚数と有効期限は出す（本体 #569）", async () => {
  mocks.api.mockResolvedValue({
    ...initial, products: [], can_purchase: false, sales_open: false, balance: { available: 3, reserved: 0 },
    next_expiry: { expires_at: "2027-05-01T00:00:00Z", available: 3 },
    purchase_block_reason: "チケットの販売は準備中です",
  });
  await render();
  expect(container.textContent).toContain("チケットの販売は準備中です");
  expect(container.textContent).toContain("チケットの残り 3枚");
  expect(container.textContent).toContain("いちばん早い有効期限");
  expect(reviewButton()).toBeUndefined();
});
