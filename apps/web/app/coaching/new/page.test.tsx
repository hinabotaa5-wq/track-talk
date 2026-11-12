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
  expect(container.textContent).toContain("未成年の方は保護者に許可を取ってください");
  const box = container.querySelector<HTMLInputElement>('input[type="checkbox"]');
  expect(box?.checked).toBe(false);
  expect(box?.closest("label")?.textContent).toBe("未成年の方は保護者に許可を取ってください");
});

it("18歳以上にも購入前チェックを出す", async () => {
  mocks.me = { demo: true, birth_month: "1995-01", is_minor: false, guardian: null, push_devices: 0 };
  await render();
  expect(container.querySelector("form")).not.toBeNull();
  expect(container.querySelector('input[type="checkbox"]')).not.toBeNull();
  expect(container.textContent).toContain("未成年の方は保護者に許可を取ってください");
});

it("支払い方法はカードだけを出し、PayPay の案内は出さない", async () => {
  mocks.me = { demo: true, birth_month: "1995-01", is_minor: false, guardian: null, push_devices: 0 };
  await render();
  const payment = [...container.querySelectorAll("fieldset")].find((f) => f.querySelector("legend")?.textContent?.startsWith("お支払い"));
  const choices = [...(payment?.querySelectorAll("button") ?? [])].map((b) => b.textContent);
  expect(choices).toEqual(["カード・Apple Pay・Google Pay"]);
  expect(payment?.querySelector("legend")?.textContent).toBe("お支払い方法");
  expect(container.textContent).not.toContain("販売者:");
  expect(container.querySelector('a[href="/terms"], a[href="/legal/commerce"]')).toBeNull();
  expect(container.textContent).not.toContain("料金は返答が返ってきた時点で確定し");
  expect(container.textContent).toContain("送ってから24時間以内に返ってきます。");
  expect(container.textContent).not.toContain("2026/11/06");
  expect(container.textContent).toContain("0/1000文字");
  expect(container.querySelector("textarea")?.hasAttribute("placeholder")).toBe(false);
  expect(container.textContent).not.toContain("PayPay");
});

it("デモの招待コード画面には販売者の表記とリンクを出さない", async () => {
  mocks.me = { demo: false, review_open: false, birth_month: "1995-01", is_minor: false, guardian: null, push_devices: 0 };
  await render();
  expect(container.querySelector("h1")?.textContent).toBe("動画添削を頼む");
  expect(container.textContent).not.toContain("コーチを選んで、動画を送ります");
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
    if (path === "/v1/me/coaching/tickets") return { balance: { available: 2, reserved: 0 }, next_expiry: { expires_at: "2027-05-01T00:00:00Z", available: 2 }, can_purchase: canPurchase };
    if (path.startsWith("/v1/coaching/estimate")) return { price_yen: 1500, payments: true, due_at: "2026-11-06T00:00:00Z" };
    throw new Error(`unexpected ${path}`);
  });
  await render();
}

it("チケットを持っている人には、申込画面に残りの枚数といちばん早い有効期限を出し、チケット払いを選べる（本体 #569）", async () => {
  await renderWithTickets(false);
  expect(container.textContent).toContain("チケットの残り 2枚");
  expect(container.textContent).toContain("いちばん早い有効期限");
  expect(container.querySelector('a[href="/me/coaching/tickets?return_to=%2Fcoaching%2Fnew"]')?.textContent).toContain("コーチングチケット");
  expect(container.textContent).toContain("チケット（1枚）");
});

it("販売中のチケット持ちには購入・確認の入口を残す", async () => {
  await renderWithTickets(true);
  expect(container.querySelector('a[href="/me/coaching/tickets?return_to=%2Fcoaching%2Fnew"]')?.textContent).toContain("チケットを購入・確認する");
});

it("支払いのある環境では、招待コードなしで申込みの画面を出す（招待コードのゲートを外した。本体 #569）", async () => {
  mocks.me = { demo: false, review_open: true, birth_month: "1995-01", is_minor: false, guardian: null, push_devices: 0 };
  await render();
  expect(container.querySelector("form")).not.toBeNull();
  expect(container.querySelector('input[aria-label="招待コード"]')).toBeNull();
  expect(container.textContent).not.toContain("フォーム添削は準備中です");
});

it("チケットがなく販売中なら、6枚2,500円のチケットを単発の料金と並べて案内する（通常価格や打ち消し線は使わない）", async () => {
  mocks.me = { demo: false, review_open: true, birth_month: "1995-01", is_minor: false, guardian: null, push_devices: 0 };
  mocks.api.mockImplementation(async (path: string) => {
    if (path === "/v1/me/coaching") return mocks.me;
    if (path.startsWith("/v1/coaching/nominees")) return { coaches: [] };
    if (path === "/v1/me/coaching/tickets") return { balance: { available: 0, reserved: 0 }, can_purchase: true };
    if (path.startsWith("/v1/coaching/estimate")) return { price_yen: 500, payments: true, due_at: "2026-11-06T00:00:00Z" };
    throw new Error(`unexpected ${path}`);
  });
  await render();
  const link = container.querySelector('a[href="/me/coaching/tickets?return_to=%2Fcoaching%2Fnew"]');
  expect(link?.textContent).toContain("チケット6枚 2,500円（税込）");
  expect(link?.textContent).toContain("1回あたり約417円（1回ずつ払うと500円）");
  expect(link?.textContent).toContain("有効期限は決済から6か月");
  expect(container.textContent).not.toContain("通常価格");
  expect(container.querySelector("s, del")).toBeNull();
});

it("送る前に最終確認を出し、確定を押すまで動画を上げない", async () => {
  mocks.me = { demo: false, review_open: true, birth_month: "1995-01", is_minor: false, guardian: null, push_devices: 0 };
  mocks.api.mockImplementation(async (path: string) => {
    if (path === "/v1/me/coaching") return mocks.me;
    if (path.startsWith("/v1/coaching/nominees")) return { coaches: [{ id: "c1", nickname: "テストコーチ", photo_url: "", events: ["100m"], years: 3, best_record: "", achievements: "" }] };
    if (path === "/v1/me/coaching/tickets") return { balance: { available: 0, reserved: 0 }, can_purchase: false };
    if (path.startsWith("/v1/coaching/estimate")) return { price_yen: 500, payments: true, due_at: "2026-11-06T00:00:00Z" };
    throw new Error(`unexpected ${path}`);
  });
  await render();
  const radio = container.querySelector<HTMLInputElement>('input[type="radio"]');
  if (radio) await act(async () => { radio.click(); });
  else {
    const pick = [...container.querySelectorAll("button")].find((b) => b.textContent?.includes("テストコーチ"));
    await act(async () => { pick?.click(); });
  }
  const textarea = container.querySelector("textarea")!;
  async function enterQuestion(value: string) {
    await act(async () => {
      const setter = Object.getOwnPropertyDescriptor(HTMLTextAreaElement.prototype, "value")!.set!;
      setter.call(textarea, value);
      textarea.dispatchEvent(new Event("input", { bubbles: true }));
    });
  }
  await enterQuestion("こちらも見てください ｈｔｔｐｓ：／／example.com");
  const input = container.querySelector<HTMLInputElement>('input[type="file"]')!;
  const file = new File(["x"], "run.mp4", { type: "video/mp4" });
  Object.defineProperty(input, "files", { value: [file] });
  await act(async () => { input.dispatchEvent(new Event("change", { bubbles: true })); });
  await act(async () => {});
  await act(async () => container.querySelector<HTMLInputElement>('input[type="checkbox"]')!.click());
  const submit = container.querySelector<HTMLButtonElement>('button[type="submit"]')!;
  expect(submit.textContent).toBe("申込み内容を確認する");
  const contactMessage = "外部の連絡先（URL・メール・SNS IDなど）は送れません。";
  expect(container.textContent).not.toContain(contactMessage);
  await act(async () => { submit.form!.requestSubmit(); });
  expect(container.querySelector("[role=alert]")?.textContent).toBe(contactMessage);
  expect(textarea.getAttribute("aria-invalid")).toBe("true");
  expect(document.activeElement).toBe(textarea);
  expect(container.querySelector('section[aria-labelledby="purchase-confirm-heading"]')).toBeNull();
  expect(mocks.api.mock.calls.some(([path]) => String(path).startsWith("/v1/uploads"))).toBe(false);

  await enterQuestion("後半に失速します。200mのラップは12.1 12.3です");
  expect(container.textContent).not.toContain(contactMessage);
  await act(async () => { submit.form!.requestSubmit(); });
  const confirm = container.querySelector('section[aria-labelledby="purchase-confirm-heading"]');
  expect(confirm?.textContent).toContain("動画添削1回（動画1本・担当 テストコーチ）");
  expect(confirm?.textContent).toContain("500円（税込）");
  expect(confirm?.textContent).toContain("カード・Apple Pay・Google Pay");
  expect(confirm?.textContent).toContain("申込みから24時間以内");
  expect(confirm?.textContent).not.toContain("2026/11/06");
  expect(confirm?.querySelector('a[href="/legal/commerce"]')?.textContent).toBe("特定商取引法に基づく表記");
  expect(confirm?.textContent).toContain("コーチが返却した時に確定します");
  expect(confirm?.textContent).toContain("確保を取り消すので代金はかかりません");
  expect(mocks.api.mock.calls.some(([path]) => String(path).startsWith("/v1/uploads"))).toBe(false);
  expect([...confirm!.querySelectorAll("button")].map((b) => b.textContent)).toEqual(["確定してお支払いへ（500円）", "戻って直す"]);
});
