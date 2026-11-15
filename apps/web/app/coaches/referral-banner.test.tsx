/** @vitest-environment jsdom */
import { act } from "react";
import { createRoot, type Root } from "react-dom/client";
import { afterEach, beforeEach, expect, it, vi } from "vitest";
import { installEnglish, setActiveLocale } from "@/lib/i18n";
import { en } from "@/lib/i18n-en";

// 本体 #739 のバナー部品の描画テスト。CoachesView 側の順序もここで固定する。
const apiFetch = vi.fn();
const auth = { token: "tok" as string | null };
const params = { tab: null as string | null };
vi.mock("next/navigation", () => ({
  useRouter: () => ({ replace: vi.fn() }),
  useSearchParams: () => ({ get: () => params.tab }),
  usePathname: () => "/coaches",
}));
vi.mock("@/lib/api", async (original) => ({ ...await original<typeof import("@/lib/api")>(), apiFetch: (...args: unknown[]) => apiFetch(...args) }));
vi.mock("@/components/auth-provider", () => ({ useAuth: () => ({ token: auth.token, loading: false }) }));
vi.mock("./coach-list", () => ({ CoachList: () => <div>coach-list</div> }));
vi.mock("./support-plan-list", () => ({ SupportPlanList: () => <div>support-plan-list</div> }));
const { ReferralBanner } = await import("./referral-banner");
const { CoachesView } = await import("./page-view");

const HEADLINE_JA = "友達2人に紹介すると、1回無料！";
const BODY_JA = "1回プランの都度払いやチケットの購入で、紹介IDが発行されます。すでに有効な紹介IDをお持ちの場合や、今回の購入が友達の紹介IDでカウントされた場合、新しいIDは発行されません。あなたのIDで友達2人が購入すると、紹介してくれた方に無料チケットがプレゼントされます。";
const HEADLINE_EN = "Refer two friends and get a free review!";
const BODY_EN = "Paying for a One-time Plan as you go or buying tickets can earn you a referral code. You won't get a new code if you already have an active code or if this purchase counts toward a friend's referral reward. When two friends purchase using your code, you receive a free ticket.";

let root: Root;
let container: HTMLDivElement;
beforeEach(() => {
  (globalThis as typeof globalThis & { IS_REACT_ACT_ENVIRONMENT: boolean }).IS_REACT_ACT_ENVIRONMENT = true;
  container = document.createElement("div"); document.body.appendChild(container); root = createRoot(container);
  apiFetch.mockReset(); auth.token = "tok"; params.tab = null;
  apiFetch.mockResolvedValue({ application: null });
});
afterEach(async () => { await act(async () => root.unmount()); container.remove(); setActiveLocale("ja"); });

it("見出しと本文を正本どおりに表示する", async () => {
  setActiveLocale("ja");
  await act(async () => root.render(<ReferralBanner />));
  const text = container.textContent ?? "";
  expect(text).toContain(HEADLINE_JA);
  expect(text).toContain(BODY_JA);
});

it("バナー全体が /me へのリンクである", async () => {
  setActiveLocale("ja");
  await act(async () => root.render(<ReferralBanner />));
  const link = container.querySelector('a[href="/me"]');
  expect(link).not.toBeNull();
  expect(link?.getAttribute("aria-label")).toBe(HEADLINE_JA);
});

it("黄色地に濃色文字で、見出しは太字・閉じるボタンなし", async () => {
  setActiveLocale("ja");
  await act(async () => root.render(<ReferralBanner />));
  const link = container.querySelector('a[href="/me"]');
  expect(link?.className).toContain("bg-yellow-400");
  expect(link?.className).toContain("text-slate-900");
  const headline = [...(container.querySelectorAll("span") ?? [])].find((el) => el.textContent === HEADLINE_JA);
  expect(headline?.className).toContain("font-bold");
  expect(container.querySelector("button")).toBeNull();
  expect(link?.textContent).not.toContain("×");
});

it("CoachesView ではバナーがタブ行より前に描画される", async () => {
  setActiveLocale("ja");
  await act(async () => root.render(<CoachesView />));
  await act(async () => { await Promise.resolve(); });
  const html = container.innerHTML;
  const bannerAt = html.indexOf(HEADLINE_JA);
  const tabsAt = html.indexOf("coach-list");
  expect(bannerAt).toBeGreaterThanOrEqual(0);
  expect(tabsAt).toBeGreaterThanOrEqual(0);
  expect(bannerAt).toBeLessThan(tabsAt);
  // DOM 順序でもバナーのリンクがタブより前にある。
  const link = container.querySelector('a[href="/me"]');
  const tabs = container.querySelector('[role="tabpanel"], [role="tablist"]');
  expect(link).not.toBeNull();
  expect(tabs).not.toBeNull();
  expect(link!.compareDocumentPosition(tabs!)).toBe(Node.DOCUMENT_POSITION_FOLLOWING);
});

it("英語表示では正本の英訳が出る", async () => {
  installEnglish(en);
  setActiveLocale("en");
  await act(async () => root.render(<ReferralBanner />));
  const text = container.textContent ?? "";
  expect(text).toContain(HEADLINE_EN);
  expect(text).toContain(BODY_EN);
  expect(text).not.toContain(HEADLINE_JA);
  expect(text).not.toContain(BODY_JA);
});
