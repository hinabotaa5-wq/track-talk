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

// 本体 #1031：紹介特典を ¥500 クーポンに変え、バナー・説明ページ・紹介カードの見出しを揃える。
// 旧見出し（1回無料・1,000円無料・無料チケット）はどこにも残さない。
const HEADLINE_JA = "2人に紹介すると500円分無料！";
const OLD_HEADLINES_JA = ["2人に紹介すると、1回(1000円)無料！", "友達2人に紹介すると、1回分(1,000円)無料！", "友達2人に紹介すると、1回無料！"];
const BODY_JA = "購入すると、紹介IDが発行されます。そのIDで友達2人が購入すると、紹介してくれた方に¥500クーポンがプレゼントされます。";
const HEADLINE_EN = "Refer 2 friends, get ¥500 off!";
const OLD_HEADLINES_EN = ["Refer 2 people, get 1 free (¥1,000)!", "Refer two friends and get one free review (¥1,000)!", "Refer two friends and get a free review!"];
const BODY_EN = "When you make a purchase, you get a referral code. When 2 friends buy with that code, the person who referred them gets a ¥500 coupon.";

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
  for (const old of OLD_HEADLINES_JA) expect(text).not.toContain(old);
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
  expect(text).not.toContain(BODY_JA);
  for (const old of OLD_HEADLINES_EN) expect(text).not.toContain(old);
  expect(container.querySelector('a[href="/referral"]')?.getAttribute("aria-label")).toBe(HEADLINE_EN);
});

it("バナー・説明ページ・紹介カードは同じ ¥500 クーポンの見出しで、旧見出しと「無料チケット」は残さない（本体 #1031）", () => {
  for (const old of OLD_HEADLINES_JA) expect(Object.hasOwn(en, old)).toBe(false);
  for (const old of OLD_HEADLINES_EN) expect(Object.values(en)).not.toContain(old);
  expect(en[HEADLINE_JA]).toBe(HEADLINE_EN);
  expect(en[BODY_JA]).toBe(BODY_EN);
  for (const file of ["app/coaches/referral-banner.tsx", "app/referral/page-view.tsx", "components/coaching-referral.tsx"]) {
    const source = readFileSync(path.resolve(process.cwd(), file), "utf8");
    expect(source).toContain(HEADLINE_JA);
    for (const old of OLD_HEADLINES_JA) expect(source).not.toContain(old);
    expect(source).not.toMatch(/L\("[^"]*無料チケット/);
  }
});
