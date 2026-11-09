/** @vitest-environment jsdom */
import { act, type ReactNode } from "react";
import { createRoot, type Root } from "react-dom/client";
import { afterEach, beforeEach, expect, it, vi } from "vitest";
import NewReviewRequestPage from "./page";

const mocks = vi.hoisted(() => ({ api: vi.fn(), me: { demo: true, birth_month: "2011-01", is_minor: true, guardian: null, push_devices: 0 } as Record<string, unknown> }));
vi.mock("next/link", () => ({ default: ({ href, children }: { href: string; children: ReactNode }) => <a href={href}>{children}</a> }));
vi.mock("next/navigation", () => ({ useRouter: () => ({ replace: vi.fn(), push: vi.fn() }) }));
vi.mock("@/components/auth-provider", () => ({ useAuth: () => ({ session: {}, token: "token", loading: false }) }));
vi.mock("@/lib/api", async (original) => ({ ...await original<typeof import("@/lib/api")>(), apiFetch: (...args: unknown[]) => mocks.api(...args) }));

let container: HTMLDivElement;
let root: Root;
beforeEach(() => {
  (globalThis as typeof globalThis & { IS_REACT_ACT_ENVIRONMENT: boolean }).IS_REACT_ACT_ENVIRONMENT = true;
  container = document.createElement("div"); document.body.appendChild(container); root = createRoot(container);
  mocks.api.mockReset();
  mocks.api.mockImplementation(async (path: string) => {
    if (path === "/v1/me/coaching") return mocks.me;
    if (path.startsWith("/v1/coaching/nominees")) return { coaches: [] };
    if (path === "/v1/me/coaching/tickets") return { balance: { available: 0, reserved: 0 }, can_purchase: false };
    if (path.startsWith("/v1/coaching/estimate")) return { price_yen: 1500, payments: true, due_at: "2026-11-06T00:00:00Z" };
    throw new Error(`unexpected ${path}`);
  });
});
afterEach(async () => { await act(async () => root.unmount()); container.remove(); });

async function render() {
  await act(async () => root.render(<NewReviewRequestPage />));
  await act(async () => {});
}

it("18歳未満は保護者の同意がなくても依頼の画面を開け、支払いの前に保護者の許可のチェックが出る", async () => {
  mocks.me = { demo: true, birth_month: "2011-01", is_minor: true, guardian: null, push_devices: 0 };
  await render();
  expect(container.querySelector("form")).not.toBeNull();
  expect(container.textContent).not.toContain("保護者の方の同意が必要です");
  expect(container.textContent).toContain("未成年の方は、購入する前に保護者の許可を得てください。");
  const box = container.querySelector<HTMLInputElement>('input[type="checkbox"]');
  expect(box?.checked).toBe(false);
  expect(box?.closest("label")?.textContent).toBe("保護者に許可をもらいました");
});

it("18歳以上には、保護者の許可のチェックを出さない", async () => {
  mocks.me = { demo: true, birth_month: "1995-01", is_minor: false, guardian: null, push_devices: 0 };
  await render();
  expect(container.querySelector("form")).not.toBeNull();
  expect(container.querySelector('input[type="checkbox"]')).toBeNull();
  expect(container.textContent).not.toContain("保護者に許可をもらいました");
});

it("支払い方法はカードだけを出し、PayPay は対応予定の案内だけを出す", async () => {
  mocks.me = { demo: true, birth_month: "1995-01", is_minor: false, guardian: null, push_devices: 0 };
  await render();
  const payment = [...container.querySelectorAll("fieldset")].find((f) => f.querySelector("legend")?.textContent?.startsWith("お支払い"));
  const choices = [...(payment?.querySelectorAll("button") ?? [])].map((b) => b.textContent);
  expect(choices).toEqual(["カード・Apple Pay・Google Pay"]);
  expect(container.textContent).toContain("販売者: TrackTalk の運営者");
  expect(container.querySelector('a[href="/legal/commerce"]')).not.toBeNull();
  expect(container.textContent).toContain("現在、お支払いはクレジットカードのみとなります。PayPayでのお支払いにも対応予定です（時期は未定です）。対応が整い次第、この画面でご案内します。");
});

it("デモの招待コード画面には販売者の表記とリンクを出さない", async () => {
  mocks.me = { demo: false, birth_month: "1995-01", is_minor: false, guardian: null, push_devices: 0 };
  await render();
  expect(container.querySelector("h1")?.textContent).toBe("動画添削を頼む");
  expect(container.textContent).toContain("コーチを選んで、動画を送ります。料金はどのコーチでも同じです。");
  expect(container.textContent).toContain("フォーム添削は準備中です");
  expect(container.querySelector('input[aria-label="招待コード"]')).not.toBeNull();
  expect(container.querySelector('button[type="submit"]')?.textContent).toBe("デモに参加する");
  expect(container.textContent).not.toContain("販売者:");
  expect(container.querySelector('a[href="/terms"], a[href="/legal/commerce"]')).toBeNull();
});

async function renderWithTickets(canPurchase: boolean) {
  mocks.me = { demo: true, birth_month: "1995-01", is_minor: false, guardian: null, push_devices: 0 };
  mocks.api.mockImplementation(async (path: string) => {
    if (path === "/v1/me/coaching") return mocks.me;
    if (path.startsWith("/v1/coaching/nominees")) return { coaches: [] };
    if (path === "/v1/me/coaching/tickets") return { balance: { available: 2, reserved: 0 }, can_purchase: canPurchase };
    if (path.startsWith("/v1/coaching/estimate")) return { price_yen: 1500, payments: true, due_at: "2026-11-06T00:00:00Z" };
    throw new Error(`unexpected ${path}`);
  });
  await render();
}

it("チケットを持っていても申込画面に残高は出さず、チケット払いの選択肢は残す", async () => {
  await renderWithTickets(false);
  expect(container.textContent).not.toContain("チケット残高");
  expect(container.textContent).not.toContain("残高・履歴");
  expect(container.querySelector('a[href="/me/coaching/tickets?return_to=%2Fcoaching%2Fnew"]')?.textContent).toContain("コーチングチケット");
  expect(container.textContent).toContain("チケット（2枚）");
});

it("販売中のチケット持ちには購入・確認の入口を残す", async () => {
  await renderWithTickets(true);
  expect(container.textContent).not.toContain("チケット残高");
  expect(container.querySelector('a[href="/me/coaching/tickets?return_to=%2Fcoaching%2Fnew"]')?.textContent).toContain("チケットを購入・確認する");
});
