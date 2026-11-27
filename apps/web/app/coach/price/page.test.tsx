/** @vitest-environment jsdom */
import { act } from "react";
import { createRoot, type Root } from "react-dom/client";
import { afterEach, beforeEach, expect, it, vi } from "vitest";
import { installEnglish, setActiveLocale } from "@/lib/i18n";
import { en } from "@/lib/i18n-en";
import type { CoachPrices } from "@/lib/coach-price";
import CoachPricePage from "./page";

// 価格の申請画面（本体 #1052 T1・T2・T3・T6）。
const replace = vi.fn();
vi.mock("next/navigation", () => ({ useRouter: () => ({ replace, push: vi.fn() }) }));
const auth = vi.hoisted(() => ({ session: { user: { id: "coach" } } as unknown, token: "token" as string | null, loading: false }));
vi.mock("@/components/auth-provider", () => ({ useAuth: () => auth }));
vi.mock("@/lib/features", async (original) => ({ ...await original<typeof import("@/lib/features")>(), coachingEnabled: true }));
const apiFetch = vi.fn();
vi.mock("@/lib/api", async (original) => ({ ...await original<typeof import("@/lib/api")>(), apiFetch: (...args: unknown[]) => apiFetch(...args) }));

function prices(overrides: { standard?: Partial<CoachPrices["plans"][number]>; premium?: Partial<CoachPrices["plans"][number]> } = {}): CoachPrices {
  return {
    min_price_yen: 500,
    default_price_yen: 1000,
    plans: [
      { plan: "standard", price_yen: 1000, is_default: true, pending: null, approved: null, ...overrides.standard },
      { plan: "premium", price_yen: null, is_default: false, pending: null, approved: null, ...overrides.premium },
    ],
  };
}

function application(plan: "standard" | "premium", price: number, status: "pending" | "approved") {
  return { id: `${plan}-${price}`, plan, price_yen: price, source: "price" as const, status, created_at: "2026-11-26T00:00:00Z", updated_at: "2026-11-26T00:00:00Z", approved_at: status === "approved" ? "2026-11-26T01:00:00Z" : null, applied_at: null };
}

let root: Root;
let container: HTMLDivElement;
beforeEach(() => {
  vi.stubGlobal("IS_REACT_ACT_ENVIRONMENT", true);
  container = document.createElement("div");
  document.body.appendChild(container);
  root = createRoot(container);
  apiFetch.mockReset();
});
afterEach(async () => {
  await act(async () => root.unmount());
  container.remove();
  setActiveLocale("ja");
});

function section(plan: "スタンダード" | "プレミアム" | "Standard" | "Premium") {
  return container.querySelector(`section[aria-label="${plan}"]`) as HTMLElement;
}

async function typeInto(input: HTMLInputElement, value: string) {
  const setter = Object.getOwnPropertyDescriptor(HTMLInputElement.prototype, "value")!.set!;
  await act(async () => {
    setter.call(input, value);
    input.dispatchEvent(new Event("input", { bubbles: true }));
  });
}

async function submit(form: HTMLFormElement) {
  await act(async () => form.dispatchEvent(new Event("submit", { bubbles: true, cancelable: true })));
}

it("プランごとにいまの価格と申請の状態を出し、500円未満は送らずに止める（T3）", async () => {
  apiFetch.mockResolvedValue(prices());
  await act(async () => root.render(<CoachPricePage />));
  expect(section("スタンダード").textContent).toContain("1,000円（共通の価格）");
  expect(section("プレミアム").textContent).toContain("未設定");
  await typeInto(section("スタンダード").querySelector("input")!, "499");
  await submit(section("スタンダード").querySelector("form")!);
  expect(section("スタンダード").textContent).toContain("価格は500円以上で入力してください");
  expect(apiFetch).toHaveBeenCalledTimes(1);
});

it("申請すると承認待ちになり、金額の変更と取り下げができる（T1・T2）", async () => {
  apiFetch.mockResolvedValueOnce(prices());
  await act(async () => root.render(<CoachPricePage />));
  apiFetch.mockResolvedValueOnce(prices({ standard: { pending: application("standard", 2000, "pending") } }));
  await typeInto(section("スタンダード").querySelector("input")!, "2,000");
  await submit(section("スタンダード").querySelector("form")!);
  expect(apiFetch).toHaveBeenLastCalledWith("/v1/coach/me/prices/standard", { method: "PUT", token: "token", body: JSON.stringify({ price_yen: 2000 }) });
  expect(section("スタンダード").textContent).toContain("承認待ち2,000円");
  expect(section("スタンダード").textContent).toContain("申請を取り下げる");

  apiFetch.mockResolvedValueOnce(prices());
  const withdraw = [...section("スタンダード").querySelectorAll("button")].find((b) => b.textContent === "申請を取り下げる")!;
  await act(async () => withdraw.click());
  expect(apiFetch).toHaveBeenLastCalledWith("/v1/coach/me/prices/standard/withdraw", { method: "POST", token: "token" });
  expect(section("スタンダード").textContent).not.toContain("承認待ち");
  expect(section("スタンダード").textContent).toContain("申請を取り下げました");
});

it("承認済み・切り替え待ちの申請は取り下げられず、プレミアムは独立に出す（T2・T6）", async () => {
  apiFetch.mockResolvedValue(prices({ standard: { approved: application("standard", 3000, "approved") }, premium: { price_yen: 30000 } }));
  await act(async () => root.render(<CoachPricePage />));
  expect(section("スタンダード").textContent).toContain("切り替え待ち3,000円");
  expect(section("スタンダード").textContent).not.toContain("申請を取り下げる");
  expect(section("プレミアム").textContent).toContain("30,000円");
  expect(section("プレミアム").textContent).not.toContain("切り替え待ち");
});

it("英語表示では英語で出す（補足 S1）", async () => {
  installEnglish(en);
  setActiveLocale("en");
  apiFetch.mockResolvedValue(prices({ standard: { pending: application("standard", 2000, "pending") } }));
  await act(async () => root.render(<CoachPricePage />));
  expect(container.textContent).toContain("Current price");
  expect(container.textContent).toContain("Awaiting approval");
  expect(container.textContent).toContain("Withdraw request");
  expect(container.textContent).not.toMatch(/承認待ち|いまの価格/);
});
