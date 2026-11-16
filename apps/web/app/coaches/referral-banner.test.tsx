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
vi.mock("./coach-list", async () => {
  const { useEffect } = await import("react");
  // 取得済みの一覧として振る舞う（本体 #804：初回取得が終わったら settled を知らせる）。
  return { CoachList: ({ onSettled }: { onSettled?: () => void }) => { useEffect(() => { onSettled?.(); }, [onSettled]); return <div>coach-list</div>; } };
});
vi.mock("./support-plan-list", async () => {
  const { useEffect } = await import("react");
  return { SupportPlanList: ({ onSettled }: { onSettled?: () => void }) => { useEffect(() => { onSettled?.(); }, [onSettled]); return <div>support-plan-list</div>; } };
});
const { ReferralBanner } = await import("./referral-banner");
const { CoachesView } = await import("./page-view");

const HEADLINE_JA = "友達2人に紹介すると、1回無料！";
const BODY_JA = "購入すると、紹介IDが発行されます。そのIDで友達2人が購入すると、紹介してくれた方に無料チケットがプレゼントされます。";
const HEADLINE_EN = "Refer two friends and get a free review!";
const BODY_EN = "A referral code is issued when you make a purchase. When two friends purchase using that code, the person who referred them receives a free ticket.";

let root: Root;
let container: HTMLDivElement;
beforeEach(() => {
  (globalThis as typeof globalThis & { IS_REACT_ACT_ENVIRONMENT: boolean }).IS_REACT_ACT_ENVIRONMENT = true;
  container = document.createElement("div"); document.body.appendChild(container); root = createRoot(container);
  apiFetch.mockReset(); auth.token = "tok"; params.tab = null;
  apiFetch.mockResolvedValue({ application: null });
});
afterEach(async () => { await act(async () => root.unmount()); container.remove(); setActiveLocale("ja"); });

it("見出しだけを表示し、本文は表示しない", async () => {
  setActiveLocale("ja");
  await act(async () => root.render(<ReferralBanner />));
  const text = container.textContent ?? "";
  expect(text).toContain(HEADLINE_JA);
  expect(text).not.toContain(BODY_JA);
});

it("バナー全体が /referral へのリンクである", async () => {
  setActiveLocale("ja");
  await act(async () => root.render(<ReferralBanner />));
  const link = container.querySelector('a[href="/referral"]');
  expect(link).not.toBeNull();
  expect(link?.getAttribute("aria-label")).toBe(HEADLINE_JA);
  expect(container.querySelector('a[href="/me"]')).toBeNull();
});

it("黄色地に濃色文字で、見出しは太字・閉じるボタンなし", async () => {
  setActiveLocale("ja");
  await act(async () => root.render(<ReferralBanner />));
  const link = container.querySelector('a[href="/referral"]');
  expect(link?.className).toContain("bg-yellow-400/70");
  expect(link?.className).toContain("text-slate-900");
  const headline = [...(container.querySelectorAll("span") ?? [])].find((el) => el.textContent === HEADLINE_JA && el.querySelector("span") === null);
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
  const link = container.querySelector('a[href="/referral"]');
  const tabs = container.querySelector('[role="tabpanel"], [role="tablist"]');
  expect(link).not.toBeNull();
  expect(tabs).not.toBeNull();
  expect(link!.compareDocumentPosition(tabs!)).toBe(Node.DOCUMENT_POSITION_FOLLOWING);
});

it("英語表示では見出しの英訳だけが出て、本文は出ない", async () => {
  installEnglish(en);
  setActiveLocale("en");
  await act(async () => root.render(<ReferralBanner />));
  const text = container.textContent ?? "";
  expect(text).toContain(HEADLINE_EN);
  expect(text).not.toContain(BODY_EN);
  expect(text).not.toContain(HEADLINE_JA);
  expect(text).not.toContain(BODY_JA);
});
