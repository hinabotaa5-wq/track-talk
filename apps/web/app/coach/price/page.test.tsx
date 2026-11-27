/** @vitest-environment jsdom */
import { act } from "react";
import { createRoot, type Root } from "react-dom/client";
import { afterEach, beforeEach, expect, it, vi } from "vitest";
import { installEnglish, setActiveLocale } from "@/lib/i18n";
import { en } from "@/lib/i18n-en";
import type { CoachPrices } from "@/lib/coach-price";
import type { CoachOffers } from "@/lib/coach-offer";
import CoachPricePage from "./page";

// 価格の申請画面（本体 #1052 T1・T2・T3・T6）。
const replace = vi.fn();
vi.mock("next/navigation", () => ({ useRouter: () => ({ replace, push: vi.fn() }) }));
const auth = vi.hoisted(() => ({ session: { user: { id: "coach" } } as unknown, token: "token" as string | null, loading: false }));
vi.mock("@/components/auth-provider", () => ({ useAuth: () => auth }));
vi.mock("@/lib/features", async (original) => ({ ...await original<typeof import("@/lib/features")>(), coachingEnabled: true }));
const apiFetch = vi.fn();
// 添削項目・見本動画の節（本体 #1086）の呼び出しは offersFetch に分け、価格のテストの呼び出し回数を変えない。
const offersFetch = vi.fn();
vi.mock("@/lib/api", async (original) => ({
  ...await original<typeof import("@/lib/api")>(),
  apiFetch: (...args: unknown[]) => (String(args[0]).startsWith("/v1/coach/me/offers") || String(args[0]).startsWith("/v1/uploads") ? offersFetch(...args) : apiFetch(...args)),
}));

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

function offers(overrides: Partial<CoachOffers> = {}): CoachOffers {
  return { items: [], demo_videos: [], pending: null, approved: null, max_items: 5, max_item_chars: 30, max_demo_videos: 3, ...overrides };
}

function offerApplication(status: "pending" | "approved", items: string[], videos: { url: string; public_id: string }[] = []) {
  return { id: `offer-${status}`, items, demo_videos: videos, status, created_at: "2026-11-27T00:00:00Z", updated_at: "2026-11-27T00:00:00Z", approved_at: status === "approved" ? "2026-11-27T01:00:00Z" : null, applied_at: null };
}

let root: Root;
let container: HTMLDivElement;
beforeEach(() => {
  vi.stubGlobal("IS_REACT_ACT_ENVIRONMENT", true);
  container = document.createElement("div");
  document.body.appendChild(container);
  root = createRoot(container);
  apiFetch.mockReset();
  offersFetch.mockReset();
  offersFetch.mockResolvedValue(offers());
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

// 本体 #1086 W1：価格の画面に「添削項目・見本動画」の節があり、状態の表示・申請・上書き・取り下げができる。
function offerSection() {
  return container.querySelector('section[aria-label="添削項目・見本動画"]') as HTMLElement;
}

it("W1: 添削項目・見本動画の節に、いまの内容・承認待ち・切り替え待ちと、入力例を出す", async () => {
  apiFetch.mockResolvedValue(prices());
  offersFetch.mockResolvedValue(offers({
    items: ["接地の分析"],
    demo_videos: [{ url: "https://example.com/now.mp4", public_id: "track-talk-coaching/now" }],
    approved: offerApplication("approved", ["接地の分析", "腕振りの分析"]),
    pending: offerApplication("pending", ["スタート局面の詳しい分析"], [{ url: "https://example.com/p.mp4", public_id: "track-talk-coaching/p" }]),
  }));
  await act(async () => root.render(<CoachPricePage />));
  const sec = offerSection();
  expect(sec).not.toBeNull();
  expect(offersFetch).toHaveBeenCalledWith("/v1/coach/me/offers", { token: "token" });
  expect(sec.querySelector('[data-testid="offer-current"]')?.textContent).toContain("接地の分析");
  expect(sec.querySelector('[data-testid="offer-current"]')?.textContent).toContain("見本動画 1本");
  expect(sec.querySelector('[data-testid="offer-approved"]')?.textContent).toContain("腕振りの分析");
  expect(sec.querySelector('[data-testid="offer-pending"]')?.textContent).toContain("スタート局面の詳しい分析");
  // 固定の4項目は変更できない項目として出す。
  expect(sec.textContent).toContain("すべての回答に入る項目（変更できません）");
  for (const fixed of ["良い点", "改善点", "おすすめのドリル", "コーチなら明日から何をするか"]) expect(sec.textContent).toContain(fixed);
  // 承認待ちがあれば、その内容が編集欄の初期値になる。入力例は運営者指定の文言（補足2 T2）。
  const inputs = [...sec.querySelectorAll<HTMLInputElement>('input[aria-label^="追加項目"]')];
  expect(inputs.map((i) => i.value)).toEqual(["スタート局面の詳しい分析"]);
  expect(inputs[0].placeholder).toBe("例：スタート局面の詳しい分析");
  expect(sec.textContent).not.toContain("コーチだったら明日から何をするか");
  expect(sec.querySelectorAll("video").length).toBeGreaterThan(0);
});

it("W1: 追加項目と見本動画のセットを申請し、承認待ちの上書き・取り下げができる", async () => {
  apiFetch.mockResolvedValue(prices());
  offersFetch.mockResolvedValueOnce(offers({ items: [], demo_videos: [] }));
  await act(async () => root.render(<CoachPricePage />));
  // 入力例は追加項目が0件でも見える。
  expect(offerSection().textContent).toContain("例：スタート局面の詳しい分析");
  const addButton = [...offerSection().querySelectorAll("button")].find((b) => b.textContent === "追加項目を足す")!;
  await act(async () => addButton.click());
  await typeInto(offerSection().querySelector<HTMLInputElement>('input[aria-label="追加項目 1"]')!, " スタート局面の詳しい分析 ");
  offersFetch.mockResolvedValueOnce(offers({ pending: offerApplication("pending", ["スタート局面の詳しい分析"]) }));
  await submit(offerSection().querySelector("form")!);
  expect(offersFetch).toHaveBeenLastCalledWith("/v1/coach/me/offers", { method: "PUT", token: "token", body: JSON.stringify({ items: ["スタート局面の詳しい分析"], demo_video_public_ids: [] }) });
  expect(offerSection().textContent).toContain("添削項目・見本動画を申請しました");
  expect(offerSection().querySelector('[data-testid="offer-pending"]')?.textContent).toContain("スタート局面の詳しい分析");

  // 上書き：項目を消して出し直す。
  const remove = [...offerSection().querySelectorAll("button")].find((b) => b.textContent === "削除")!;
  await act(async () => remove.click());
  offersFetch.mockResolvedValueOnce(offers({ pending: { ...offerApplication("pending", []), updated_at: "2026-11-27T02:00:00Z" } }));
  await submit(offerSection().querySelector("form")!);
  expect(offersFetch).toHaveBeenLastCalledWith("/v1/coach/me/offers", { method: "PUT", token: "token", body: JSON.stringify({ items: [], demo_video_public_ids: [] }) });
  expect(offerSection().textContent).toContain("申請の内容を変更しました");

  offersFetch.mockResolvedValueOnce(offers());
  const withdraw = [...offerSection().querySelectorAll("button")].find((b) => b.textContent === "申請を取り下げる")!;
  await act(async () => withdraw.click());
  expect(offersFetch).toHaveBeenLastCalledWith("/v1/coach/me/offers/withdraw", { method: "POST", token: "token" });
  expect(offerSection().textContent).toContain("申請を取り下げました");
  expect(offerSection().querySelector('[data-testid="offer-pending"]')).toBeNull();
});

it("W1: 固定項目と同じタイトル・31文字は送らずに止める", async () => {
  apiFetch.mockResolvedValue(prices());
  await act(async () => root.render(<CoachPricePage />));
  const addButton = () => [...offerSection().querySelectorAll("button")].find((b) => b.textContent === "追加項目を足す")!;
  await act(async () => addButton().click());
  await typeInto(offerSection().querySelector<HTMLInputElement>('input[aria-label="追加項目 1"]')!, "コーチだったら明日から何をするか");
  await submit(offerSection().querySelector("form")!);
  expect(offerSection().textContent).toContain("追加項目にはできません");
  await typeInto(offerSection().querySelector<HTMLInputElement>('input[aria-label="追加項目 1"]')!, "あ".repeat(31));
  await submit(offerSection().querySelector("form")!);
  expect(offerSection().textContent).toContain("30文字以内");
  expect(offersFetch).toHaveBeenCalledTimes(1);
});
