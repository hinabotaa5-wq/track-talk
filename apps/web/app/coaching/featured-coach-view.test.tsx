/** @vitest-environment jsdom */
import { act, createElement, type ReactNode } from "react";
import { createRoot, type Root } from "react-dom/client";
import { afterEach, beforeEach, expect, it, vi } from "vitest";
import { installEnglish, setActiveLocale } from "@/lib/i18n";
import { en } from "@/lib/i18n-en";

// コーチタブの特集ページ（本体 #985、#991 で並びを変更）。特集コーチ1人の詳細だけを取り、一覧・種目チップは出さない。
// 並び：紹介バナー → 切り替え（スタンダードプランのみ、本体 #1007）＋選手・コーチのボタン → コーチについて → 3ステップ → よくある質問（＋固定 CTA）。
const apiFetch = vi.fn();
const auth = { token: null as string | null };
const flags = { supportList: false };
const nav = { search: "" };
vi.mock("@/lib/api", async (original) => ({ ...await original<typeof import("@/lib/api")>(), apiFetch: (...args: unknown[]) => apiFetch(...args) }));
vi.mock("@/components/auth-provider", () => ({ useAuth: () => ({ token: auth.token, loading: false }) }));
vi.mock("@/lib/features", async (original) => ({ ...await original<typeof import("@/lib/features")>(), get supportPlanListEnabled() { return flags.supportList; } }));
vi.mock("next/navigation", () => ({ useSearchParams: () => new URLSearchParams(nav.search), usePathname: () => "/coaching" }));
vi.mock("next/link", () => ({
  default: ({ children, href, ...props }: { children: ReactNode; href: string }) => createElement("a", { href, ...props }, children),
}));
const { FeaturedCoachView, recordChips } = await import("./featured-coach-view");

const coach = {
  id: "2f6d0a8e-1111-4c2b-9a7e-3c332627b25f", nickname: "テスト太郎", nickname_en: null, photo_url: "https://img.example/c.jpg", tier: "coach",
  events: ["100m", "200m", "400m"], best_record: "100m 10.76／400m 47.54", achievements: "福岡のスプリントコーチ\n全国大会 優勝",
  years: 5, sns_urls: ["https://www.instagram.com/example_account/"], hitokoto: "一緒に速くなりましょう。", on_duty: true, country_code: "JP",
  plans: [], service_types: [], rating_count: 0, rating_average: 0,
};

let root: Root;
let container: HTMLDivElement;
beforeEach(() => {
  (globalThis as typeof globalThis & { IS_REACT_ACT_ENVIRONMENT: boolean }).IS_REACT_ACT_ENVIRONMENT = true;
  container = document.createElement("div"); document.body.appendChild(container); root = createRoot(container);
  auth.token = null; flags.supportList = false; nav.search = "";
  window.history.replaceState(null, "", "/coaching");
  apiFetch.mockReset();
  apiFetch.mockImplementation(async (path: string) => {
    if (path === "/v1/coaches/2f6d0a8e-1111-4c2b-9a7e-3c332627b25f") return { ...coach };
    throw new Error(`unexpected ${path}`);
  });
});
afterEach(async () => { await act(async () => root.unmount()); container.remove(); setActiveLocale("ja"); });
const render = async () => {
  await act(async () => root.render(<FeaturedCoachView coachID="2f6d0a8e-1111-4c2b-9a7e-3c332627b25f" />));
  await act(async () => { await Promise.resolve(); });
};
const link = (text: string) => Array.from(container.querySelectorAll("a")).find((a) => a.textContent?.includes(text));

const about = () => container.querySelector('section[aria-labelledby="featured-about"]');
const steps = () => container.querySelector('[data-testid="featured-steps"]');
const switchButton = (label: string) => [...container.querySelectorAll('[role="group"][aria-label="コーチの一覧を切り替える"] a')].find((a) => a.textContent === label) as HTMLAnchorElement | undefined;
const before = (a: Element | null | undefined, b: Element | null | undefined) => Boolean(a && b && a.compareDocumentPosition(b) & Node.DOCUMENT_POSITION_FOLLOWING);

it("紹介バナー → 切り替え → コーチについて（顔写真・記録チップ）→ 3ステップ → よくある質問 の順に出し、固定 CTA（¥1,000、特集コーチ指名で依頼へ）を出す", async () => {
  await render();
  expect(apiFetch.mock.calls.map(([path]) => path)).toEqual(["/v1/coaches/2f6d0a8e-1111-4c2b-9a7e-3c332627b25f"]);
  // T1'：ヒーロー（コーチの紹介）とキャッチコピー・ページ見出しは出さない。
  expect(container.querySelector('section[aria-label="コーチの紹介"]')).toBeNull();
  expect(container.querySelector("h1")).toBeNull();
  expect(container.textContent).not.toContain("スタートとフォームの悩みを、動画1本で。");
  // T2'：顔写真（丸型の CoachAvatar 112px）と記録・対応種目のチップは「コーチについて」の中。
  const photo = about()?.querySelector("img");
  expect(photo?.getAttribute("src")).toBe(coach.photo_url);
  expect(photo?.getAttribute("alt")).toBe("テスト太郎の顔写真");
  expect(photo?.className).toContain("rounded-full");
  expect(photo?.getAttribute("style")).toContain("width: 112px");
  expect(container.querySelector(".aspect-\\[4\\/5\\]")).toBeNull();
  expect(container.querySelectorAll("img")).toHaveLength(1);
  const chips = Array.from(about()?.querySelectorAll('ul[aria-label="記録と対応種目"] li') ?? []).map((li) => li.textContent);
  expect(chips).toEqual(["100m 10.76", "400m 47.54", "対応種目 100m・200m・400m"]);
  expect(about()?.querySelector("h2")?.textContent).toBe("テスト太郎コーチについて");
  expect(about()?.textContent).toContain("一緒に速くなりましょう。");
  expect(about()?.textContent).toContain("福岡のスプリントコーチ");
  expect(about()?.querySelector('a[href="https://www.instagram.com/example_account/"]')).not.toBeNull();
  // 3ステップは見出しなしで番号付きリストだけ。見出し「どんな添削が返ってくるか」と回答の見本は出さない。
  expect(steps()?.tagName).toBe("OL");
  expect(Array.from(steps()?.querySelectorAll("li") ?? []).map((li) => li.textContent)).toEqual(["1動画を1本送るだけ", "2コーチが動画を見て回答", "324時間以内に返却"]);
  expect(container.textContent).not.toContain("どんな添削が返ってくるか");
  expect(container.querySelector('article[aria-label="回答の見本"]')).toBeNull();
  expect(container.querySelector('[aria-labelledby="featured-sample"], #featured-sample')).toBeNull();
  expect(container.textContent).not.toContain("回答の見本");
  // 並び：紹介バナー → 切り替え → コーチについて → 3ステップ → よくある質問。
  const banner = container.querySelector('a[href="/referral"]');
  const group = container.querySelector('[role="group"][aria-label="コーチの一覧を切り替える"]');
  const faq = container.querySelector('section[aria-labelledby="featured-faq"]');
  expect(banner?.textContent).toContain("1回(1000円)無料！");
  expect(before(banner, group)).toBe(true);
  expect(before(group, about())).toBe(true);
  expect(before(about(), steps())).toBe(true);
  expect(before(steps(), faq)).toBe(true);
  expect(faq?.textContent).toContain("よくある質問");
  // 切り替えと選手・コーチの丸ボタン。選択肢はスタンダードプランだけ（本体 #1007）。
  expect(switchButton("スタンダードプラン")?.getAttribute("aria-current")).toBe("page");
  expect(switchButton("継続プラン")).toBeUndefined();
  expect(container.querySelector('a[href="/player"][aria-label="選手"]')).not.toBeNull();
  expect(container.querySelector('a[href="/coach"][aria-label="コーチ"]')).not.toBeNull();
  // 継続プランの文言を出さない。
  expect(container.textContent).not.toContain("継続プランは準備中です");
  expect(link("継続プランを見る")).toBeUndefined();
  const requestLink = link("動画添削を依頼する ¥1,000");
  expect(requestLink?.getAttribute("href")).toBe("/coaching/new?coach=2f6d0a8e-1111-4c2b-9a7e-3c332627b25f&pay=card");
  expect(requestLink?.className).toContain("bg-[#EF442D]");
  expect(requestLink?.className).not.toContain("bg-blue-600");
  const ticketLink = link("チケットを買う");
  expect(ticketLink?.getAttribute("href")).toBe("/me/coaching/tickets?return_to=%2Fcoaches%2F2f6d0a8e-1111-4c2b-9a7e-3c332627b25f");
  expect(ticketLink?.className).toContain("border-blue-400/60");
  // 一覧の部品（種目チップの絞り込み）は出さない。
  expect(container.querySelector('[role="radiogroup"]')).toBeNull();
});

it("写真が未登録でも、コーチについての先頭に頭文字の丸アバターを出す", async () => {
  apiFetch.mockImplementation(async () => ({ ...coach, photo_url: "" }));
  await render();
  expect(container.querySelector("img")).toBeNull();
  const initial = about()?.querySelector("span.rounded-full");
  expect(initial?.textContent).toBe("テ");
  expect(initial?.getAttribute("style")).toContain("width: 112px");
  expect(Array.from(about()?.querySelectorAll('ul[aria-label="記録と対応種目"] li') ?? []).map((li) => li.textContent)).toContain("対応種目 100m・200m・400m");
});

// 本体 #1007 B2：継続プランは休眠。?tab=support で開いてもスタンダードプラン側（特集コンテンツと固定 CTA）を出し、
// 「継続プランは準備中です」は出さない。一覧フラグの真偽にかかわらない。
for (const supportList of [true, false]) {
  it(`?tab=support で開いてもスタンダードプラン側を出し、継続プランの選択肢・準備中表示を出さない（一覧フラグ=${supportList}）`, async () => {
    flags.supportList = supportList;
    nav.search = "tab=support";
    await render();
    const options = [...container.querySelectorAll('[role="group"][aria-label="コーチの一覧を切り替える"] a')].map((a) => a.textContent);
    expect(options).toEqual(["スタンダードプラン"]);
    expect(switchButton("スタンダードプラン")?.getAttribute("aria-current")).toBe("page");
    expect(switchButton("スタンダードプラン")?.getAttribute("href")).toBe("/coaching");
    expect(container.textContent).not.toContain("継続プラン");
    expect(container.querySelector('a[href="/referral"]')).not.toBeNull();
    expect(about()).not.toBeNull();
    expect(steps()).not.toBeNull();
    expect(container.querySelector('[data-testid="featured-cta"]')).not.toBeNull();
    expect(link("動画添削を依頼する ¥1,000")).not.toBeUndefined();
    expect(apiFetch.mock.calls.map(([path]) => path)).toEqual(["/v1/coaches/2f6d0a8e-1111-4c2b-9a7e-3c332627b25f"]);
  });
}

it("受付停止中は、CTA を停止表示にし、依頼へ進めない", async () => {
  apiFetch.mockImplementation(async () => ({ ...coach, on_duty: false }));
  await render();
  expect(container.textContent).toContain("現在は受付を停止しています");
  expect(link("動画添削を依頼する")).toBeUndefined();
  expect(container.querySelector('[aria-disabled="true"]')?.textContent).toBe("動画添削を依頼する ¥1,000");
});

it("チケットの新規販売を止めている間は「チケットを買う」を出さない", async () => {
  auth.token = "tok";
  apiFetch.mockImplementation(async (path: string) => (path === "/v1/me/coaching/tickets" ? { can_purchase: false, balances: [] } : { ...coach }));
  await render();
  expect(link("チケットを買う")).toBeUndefined();
});

it("特集コーチが非公開なら、ほかのコーチを出さずに非公開と表示する", async () => {
  const { ApiRequestError } = await import("@/lib/api");
  apiFetch.mockImplementation(async () => { throw new ApiRequestError(404, { error: { code: "coach_not_public", message: "x" } }); });
  await render();
  expect(container.textContent).toContain("このコーチは現在公開されていません。");
});

it("英語表示では固定文を英語で出す", async () => {
  installEnglish(en);
  setActiveLocale("en");
  await render();
  expect(container.textContent).toContain("Request video feedback ¥1,000");
  expect(container.textContent).toContain("Returned within 24 hours");
  expect(container.textContent).toContain("Standard Plan");
  expect(container.textContent).toContain("FAQ");
  expect(container.textContent).not.toContain("よくある質問");
});

it("記録の欄を区切りでチップに分ける", () => {
  expect(recordChips("100m 10.76、400m 47.54\n200m 21.9")).toEqual(["100m 10.76", "400m 47.54", "200m 21.9"]);
  expect(recordChips(null)).toEqual([]);
});
