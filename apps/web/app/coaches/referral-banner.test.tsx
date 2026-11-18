/** @vitest-environment jsdom */
import { readFileSync } from "node:fs";
import path from "node:path";
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

// 本体 #949：バナーだけ短い見出しにし、説明ページ・紹介カードの見出しは維持する。
const HEADLINE_JA = "2人に紹介すると、1回(500円)無料！";
const DETAIL_HEADLINE_JA = "友達2人に紹介すると、1回分(500円)無料！";
const OLD_HEADLINE_JA = "友達2人に紹介すると、1回無料！";
const BODY_JA = "購入すると、紹介IDが発行されます。そのIDで友達2人が購入すると、紹介してくれた方に無料チケットがプレゼントされます。";
const HEADLINE_EN = "Refer 2 people, get 1 free (¥500)!";
const DETAIL_HEADLINE_EN = "Refer two friends and get one free review (¥500)!";
const OLD_HEADLINE_EN = "Refer two friends and get a free review!";
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
  expect(text).toBe(HEADLINE_JA);
  expect(text).not.toContain(DETAIL_HEADLINE_JA);
  expect(text).not.toContain(OLD_HEADLINE_JA);
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

it("バナーは56pxの高さで、横パディングと間隔を狭める", async () => {
  setActiveLocale("ja");
  await act(async () => root.render(<ReferralBanner />));
  const link = container.querySelector('a[href="/referral"]');
  for (const name of ["h-14", "px-3", "gap-2", "bg-yellow-400/70", "text-slate-900", "rounded-xl"]) {
    expect(link?.classList.contains(name)).toBe(true);
  }
  for (const name of ["h-12", "px-4", "gap-3"]) {
    expect(link?.classList.contains(name)).toBe(false);
  }
  expect(link?.classList.contains("p-4")).toBe(false);
});

it("見出しは13pxで、390px以上では16pxの1行表示にする", async () => {
  setActiveLocale("ja");
  await act(async () => root.render(<ReferralBanner />));
  const headline = container.querySelector('a[href="/referral"] > span > span');
  expect(headline?.textContent).toBe(HEADLINE_JA);
  expect(headline?.classList.contains("text-[13px]")).toBe(true);
  for (const name of ["min-[390px]:text-base", "truncate", "font-semibold", "block"]) {
    expect(headline?.classList.contains(name)).toBe(true);
  }
});

it("黄色地に濃色文字で、見出しはセミボールド・閉じるボタンなし", async () => {
  setActiveLocale("ja");
  await act(async () => root.render(<ReferralBanner />));
  const link = container.querySelector('a[href="/referral"]');
  expect(link?.className).toContain("bg-yellow-400/70");
  expect(link?.className).toContain("text-slate-900");
  const headline = [...(container.querySelectorAll("span") ?? [])].find((el) => el.textContent === HEADLINE_JA && el.querySelector("span") === null);
  expect(headline?.className).toContain("font-semibold");
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
  expect(text).not.toContain(DETAIL_HEADLINE_JA);
  expect(text).not.toContain(DETAIL_HEADLINE_EN);
  expect(text).not.toContain(BODY_JA);
  expect(text).not.toContain(OLD_HEADLINE_EN);
  expect(text).not.toContain(OLD_HEADLINE_JA);
  expect(container.querySelector('a[href="/referral"]')?.getAttribute("aria-label")).toBe(HEADLINE_EN);
});

it("バナーだけ新見出しで、説明ページ・紹介カードと辞書の旧見出しは維持する（本体 #949）", () => {
  expect(Object.hasOwn(en, OLD_HEADLINE_JA)).toBe(false);
  expect(Object.values(en)).not.toContain(OLD_HEADLINE_EN);
  expect(en[HEADLINE_JA]).toBe(HEADLINE_EN);
  expect(en[DETAIL_HEADLINE_JA]).toBe(DETAIL_HEADLINE_EN);
  const banner = readFileSync(path.resolve(process.cwd(), "app/coaches/referral-banner.tsx"), "utf8");
  expect(banner).toContain(HEADLINE_JA);
  expect(banner).not.toContain(DETAIL_HEADLINE_JA);
  expect(banner).not.toContain(OLD_HEADLINE_JA);
  for (const file of ["app/referral/page-view.tsx", "components/coaching-referral.tsx"]) {
    const source = readFileSync(path.resolve(process.cwd(), file), "utf8");
    expect(source).not.toContain(OLD_HEADLINE_JA);
    expect(source).toContain(DETAIL_HEADLINE_JA);
    expect(source).not.toContain(HEADLINE_JA);
  }
});
