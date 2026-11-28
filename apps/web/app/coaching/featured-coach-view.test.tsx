/** @vitest-environment jsdom */
import { act, createElement, type ReactNode } from "react";
import { createRoot, type Root } from "react-dom/client";
import { afterEach, beforeEach, expect, it, vi } from "vitest";
import { installEnglish, setActiveLocale } from "@/lib/i18n";
import { en } from "@/lib/i18n-en";

// コーチタブの特集ページ（本体 #985、#991 で並びを変更）。特集コーチ1人の詳細だけを取り、一覧・種目チップは出さない。
// 並び（本体 #1054）：紹介バナー → 見出し文＋選手・コーチのボタン → コーチについて → スタンダードの流れ → プレミアムの流れ（＋固定 CTA の2ボタン）。
// 上部の切り替えとよくある質問は #1054 で無くした。
const apiFetch = vi.fn();
const auth = { token: null as string | null };
const flags = { supportList: false };
const nav = { search: "" };
vi.mock("@/lib/api", async (original) => ({ ...await original<typeof import("@/lib/api")>(), apiFetch: (...args: unknown[]) => apiFetch(...args) }));
vi.mock("@/components/auth-provider", () => ({ useAuth: () => ({ token: auth.token, loading: false }) }));
vi.mock("@/lib/features", async (original) => ({ ...await original<typeof import("@/lib/features")>(), get supportPlanListEnabled() { return flags.supportList; } }));
vi.mock("next/navigation", () => ({ useSearchParams: () => new URLSearchParams(nav.search), usePathname: () => "/coaching" }));
// onNavigate は DOM に渡さず、href ごとに捕捉してテストから呼べるようにする（本体 #1041 の T2）。
const navigateHandlers = new Map<string, () => void>();
vi.mock("next/link", () => ({
  default: ({ children, href, onNavigate, ...props }: { children: ReactNode; href: string; onNavigate?: () => void }) => {
    if (onNavigate) navigateHandlers.set(href, onNavigate);
    return createElement("a", { href, ...props }, children);
  },
}));
const { FeaturedCoachView, recordChips } = await import("./featured-coach-view");
const { ScreenLoader } = await import("@/components/screen-loader");
const { clearLinkPending } = await import("@/lib/pending-link");

const coach = {
  id: "2f6d0a8e-1111-4c2b-9a7e-3c332627b25f", nickname: "テスト太郎", nickname_en: null, photo_url: "https://img.example/c.jpg", tier: "coach",
  events: ["100m", "200m", "400m"], best_record: "100m 10.76／400m 47.54", achievements: "福岡のスプリントコーチ\n全国大会 優勝",
  years: 5, sns_urls: ["https://www.instagram.com/example_account/"], hitokoto: "一緒に速くなりましょう。", on_duty: true, country_code: "JP", review_price_yen: 1000,
  plans: [], service_types: [], rating_count: 0, rating_average: 0,
};

let root: Root;
let container: HTMLDivElement;
beforeEach(() => {
  (globalThis as typeof globalThis & { IS_REACT_ACT_ENVIRONMENT: boolean }).IS_REACT_ACT_ENVIRONMENT = true;
  container = document.createElement("div"); document.body.appendChild(container); root = createRoot(container);
  auth.token = null; flags.supportList = false; nav.search = "";
  window.history.replaceState(null, "", "/coaching");
  apiFetch.mockReset(); navigateHandlers.clear(); clearLinkPending();
  apiFetch.mockImplementation(async (path: string) => {
    if (path === "/v1/coaches/2f6d0a8e-1111-4c2b-9a7e-3c332627b25f") return { ...coach };
    throw new Error(`unexpected ${path}`);
  });
});
afterEach(async () => { await act(async () => root.unmount()); container.remove(); setActiveLocale("ja"); });
const render = async () => {
  await act(async () => root.render(<FeaturedCoachView coachIDs={["2f6d0a8e-1111-4c2b-9a7e-3c332627b25f"]} />));
  await act(async () => { await Promise.resolve(); });
};
const link = (text: string) => Array.from(container.querySelectorAll("a")).find((a) => a.textContent?.includes(text));

const about = () => container.querySelector('section[aria-labelledby="featured-about"]');
const steps = () => container.querySelector('[data-testid="featured-steps"]');
const premiumSteps = () => container.querySelector('[data-testid="featured-premium-steps"]');
const lead = () => container.querySelector('[data-testid="featured-lead"]');
const stepTexts = (el: Element | null) => Array.from(el?.querySelectorAll("li") ?? []).map((li) => li.textContent);
const stepHeading = (el: Element | null) => el?.closest("section")?.querySelector("h2")?.textContent;
const cta = () => container.querySelector('[data-testid="featured-cta"]');
const faqSection = () => container.querySelector('section[aria-labelledby="featured-faq"]');
const before = (a: Element | null | undefined, b: Element | null | undefined) => Boolean(a && b && a.compareDocumentPosition(b) & Node.DOCUMENT_POSITION_FOLLOWING);

it("紹介バナー → 見出し文 → コーチについて（顔写真・記録チップ）→ スタンダードの流れ → プレミアムの流れ の順に出し、固定 CTA（¥1,000、特集コーチ指名で依頼へ）を出す", async () => {
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
  // T2（本体 #1054）：流れはスタンダード3点 → プレミアム3点の順で、それぞれ小見出し付き。見出し「どんな添削が返ってくるか」と回答の見本は出さない。
  expect(steps()?.tagName).toBe("OL");
  expect(stepTexts(steps())).toEqual(["1動画を1本送るだけ", "2コーチが動画を見て回答", "324時間以内に返却"]);
  expect(stepHeading(steps())).toBe("スタンダード");
  expect(premiumSteps()?.tagName).toBe("OL");
  expect(premiumSteps()?.className).toBe(steps()?.className);
  expect(stepTexts(premiumSteps())).toEqual(["1テーマごとに動画を送るだけ（最大6本まで）", "2コーチがまとめて見て診断", "324時間以内に回答とまとめを返却"]);
  expect(stepHeading(premiumSteps())).toBe("プレミアム");
  expect(before(steps(), premiumSteps())).toBe(true);
  expect(container.textContent).not.toContain("どんな添削が返ってくるか");
  expect(container.querySelector('article[aria-label="回答の見本"]')).toBeNull();
  expect(container.querySelector('[aria-labelledby="featured-sample"], #featured-sample')).toBeNull();
  expect(container.textContent).not.toContain("回答の見本");
  // T1（本体 #1054）：切り替えは出さず、その位置に見出し文を出す。並び：紹介バナー → 見出し文 → コーチについて → 流れ。
  const banner = container.querySelector('a[href="/referral"]');
  expect(banner?.textContent).toContain("2人に紹介すると500円分無料！");
  expect(container.querySelector('[role="group"][aria-label="コーチの一覧を切り替える"]')).toBeNull();
  expect(lead()?.textContent).toBe("フォームや練習内容を診断してもらおう！");
  expect(before(banner, lead())).toBe(true);
  expect(before(lead(), about())).toBe(true);
  expect(before(about(), steps())).toBe(true);
  expect(container.textContent).not.toContain("プレミアムは準備中です");
  // T3（本体 #1054）：よくある質問は見出しも質問も出さない。
  expect(faqSection()).toBeNull();
  expect(container.querySelector("details")).toBeNull();
  for (const text of ["よくある質問", "返却までどのくらいかかりますか？", "どんな動画を送ればいいですか？", "受付が止まっているときはどうなりますか？"]) {
    expect(container.textContent).not.toContain(text);
  }
  expect(container.querySelector('a[href="/player"][aria-label="選手"]')).not.toBeNull();
  expect(container.querySelector('a[href="/coach"][aria-label="コーチ"]')).not.toBeNull();
  // 継続プランの文言を出さない。
  expect(container.textContent).not.toContain("継続プランは準備中です");
  expect(link("継続プランを見る")).toBeUndefined();
  // 価格のないコーチ（premium_price_yen なし）はスタンダードの1ボタンだけ。
  expect(cta()?.querySelectorAll("a")).toHaveLength(1);
  const requestLink = link("スタンダード ¥1,000");
  expect(requestLink?.getAttribute("href")).toBe("/coaching/new?coach=2f6d0a8e-1111-4c2b-9a7e-3c332627b25f&pay=card");
  expect(requestLink?.className).toContain("bg-[#EF442D]");
  expect(requestLink?.className).not.toContain("bg-blue-600");
  // チケットの販売は終了した（本体 #1031）。購入の入口は出さない。
  expect(link("チケットを買う")).toBeUndefined();
  expect(container.querySelector('a[href^="/me/coaching/tickets"]')).toBeNull();
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

// T1（本体 #1054）：?tab=premium・?tab=support で開いても内容は切り替わらず、同じ特集ページを出す。
for (const search of ["tab=premium", "tab=support"]) {
  it(`?${search} で開いても切り替えずに同じ特集ページを出す`, async () => {
    nav.search = search;
    await render();
    expect(container.querySelector('[role="group"]')).toBeNull();
    expect(lead()?.textContent).toBe("フォームや練習内容を診断してもらおう！");
    expect(container.textContent).not.toContain("プレミアムは準備中です");
    expect(container.textContent).not.toContain("継続プラン");
    expect(about()).not.toBeNull();
    expect(steps()).not.toBeNull();
    expect(premiumSteps()).not.toBeNull();
    expect(cta()).not.toBeNull();
    expect(link("スタンダード ¥1,000")).not.toBeUndefined();
    expect(apiFetch.mock.calls.map(([path]) => path)).toEqual(["/v1/coaches/2f6d0a8e-1111-4c2b-9a7e-3c332627b25f"]);
  });
}

const premiumHref = "/coaching/new?plan=premium&coach=2f6d0a8e-1111-4c2b-9a7e-3c332627b25f&pay=card";

// T4（本体 #1054）
it("プレミアム価格のあるコーチは、固定 CTA に左スタンダード・右プレミアムの2ボタンを横並びで出す（API の有効価格・遷移先）", async () => {
  apiFetch.mockImplementation(async () => ({ ...coach, review_price_yen: 1200, premium_price_yen: 6000 }));
  await render();
  const buttons = [...(cta()?.querySelectorAll("a") ?? [])];
  expect(buttons.map((a) => a.textContent)).toEqual(["スタンダード ¥1,200", "プレミアム ¥6,000"]);
  expect(buttons.map((a) => a.getAttribute("href"))).toEqual(["/coaching/new?coach=2f6d0a8e-1111-4c2b-9a7e-3c332627b25f&pay=card", premiumHref]);
  expect(buttons[0].parentElement).toBe(buttons[1].parentElement);
  expect(buttons[0].parentElement?.className).toContain("flex gap-2");
  for (const a of buttons) {
    expect(a.className).toContain("flex-1");
    expect(a.hasAttribute("data-pending-link")).toBe(true);
    expect(a.hasAttribute("data-screen-loader")).toBe(true);
  }
  expect(container.textContent).not.toContain("¥1,000");
});

it("プレミアム価格が null のコーチは、プレミアムボタンを出さずスタンダードだけを全幅で出す", async () => {
  apiFetch.mockImplementation(async () => ({ ...coach, premium_price_yen: null }));
  await render();
  const buttons = [...(cta()?.querySelectorAll("a") ?? [])];
  expect(buttons.map((a) => a.textContent)).toEqual(["スタンダード ¥1,000"]);
  expect(buttons[0].className).toContain("flex-1");
  expect(cta()?.textContent).not.toContain("プレミアム");
  expect(container.querySelector(`a[href="${premiumHref}"]`)).toBeNull();
});

it("プレミアムボタンをタップしても画面を開いている幕を出す", async () => {
  apiFetch.mockImplementation(async () => ({ ...coach, premium_price_yen: 6000 }));
  await act(async () => root.render(<><FeaturedCoachView coachIDs={["2f6d0a8e-1111-4c2b-9a7e-3c332627b25f"]} /><ScreenLoader /></>));
  await act(async () => { await Promise.resolve(); });
  const onNavigate = navigateHandlers.get(premiumHref);
  expect(onNavigate).toBeTypeOf("function");
  await act(async () => onNavigate?.());
  expect(container.querySelector(".screen-loader")?.className).not.toContain("hidden");
  expect(container.querySelector('[role="status"]')?.textContent).toBe("画面を開いています");
});

// T5（本体 #1054）
it("受付停止中は、スタンダード・プレミアムの両ボタンとも無効表示にする", async () => {
  apiFetch.mockImplementation(async () => ({ ...coach, on_duty: false, premium_price_yen: 6000 }));
  await render();
  expect(container.textContent).toContain("現在は受付を停止しています");
  expect(cta()?.querySelector("a")).toBeNull();
  const disabled = [...(cta()?.querySelectorAll('[aria-disabled="true"]') ?? [])];
  expect(disabled.map((el) => [el.tagName, el.textContent])).toEqual([["SPAN", "スタンダード ¥1,000"], ["SPAN", "プレミアム ¥6,000"]]);
  expect(cta()?.querySelector("[data-pending-link], [data-screen-loader]")).toBeNull();
});

// 本体 #1088：プレミアムの購入ボタンだけ黒×ゴールドの高級仕様。スタンダードは不変。
const standardCtaClassName = "flex min-h-11 flex-1 items-center justify-center rounded-md bg-[#EF442D] px-3 text-sm font-semibold text-pure-white transition active:scale-[0.98]";
// ボタン直下は1つのインライン要素（flex で間のスペースが消えないように）。その中に名称と価格の2要素。
const premiumParts = (a: Element | undefined) => {
  expect(a?.children).toHaveLength(1);
  expect(a?.firstElementChild?.tagName).toBe("SPAN");
  return [...(a?.firstElementChild?.children ?? [])].map((el) => [el.textContent, el.className]);
};

it("プレミアムボタンだけ黒グラデーション・金の枠・影にし、スタンダードは現行のままにする（本体 #1088 T1・T2・T4）", async () => {
  apiFetch.mockImplementation(async () => ({ ...coach, review_price_yen: 1200, premium_price_yen: 6000 }));
  await render();
  const [standard, premium] = [...(cta()?.querySelectorAll("a") ?? [])];
  expect(standard.className).toBe(standardCtaClassName);
  expect(standard.children).toHaveLength(0);
  expect(premium.className).not.toContain("bg-[#EF442D]");
  for (const cls of ["bg-[linear-gradient(135deg,#0D0D0D,#2B2B2B)]", "border", "border-[#C9A227]", "shadow-[inset_0_1px_0_rgba(255,215,120,0.45),0_2px_8px_rgba(0,0,0,0.35)]",
    "min-h-11", "flex-1", "rounded-md", "px-3", "text-sm", "font-semibold", "active:scale-[0.98]"]) {
    expect(premium.className.split(" ")).toContain(cls);
  }
  expect(premium.className).not.toMatch(/animate-/);
  expect([standard.getAttribute("href"), premium.getAttribute("href")]).toEqual([ctaHref, premiumHref]);
  for (const a of [standard, premium]) {
    expect(a.hasAttribute("data-pending-link")).toBe(true);
    expect(a.hasAttribute("data-screen-loader")).toBe(true);
  }
});

it("プレミアムボタンは名称を金、価格を白の別要素で描き、全体の文字は従来どおり（本体 #1088 T3）", async () => {
  apiFetch.mockImplementation(async () => ({ ...coach, review_price_yen: 1200, premium_price_yen: 6000 }));
  await render();
  const premium = cta()?.querySelectorAll("a")[1];
  expect(premium?.textContent).toBe("プレミアム ¥6,000");
  expect(premiumParts(premium)).toEqual([["プレミアム", "tracking-[0.06em] text-[#E8C766]"], ["¥6,000", "text-pure-white"]]);
});

it("英語表示でもプレミアムボタンは名称 Premium を金、価格を白で描く（本体 #1088 T3）", async () => {
  installEnglish(en);
  setActiveLocale("en");
  apiFetch.mockImplementation(async () => ({ ...coach, premium_price_yen: 6000 }));
  await render();
  const premium = cta()?.querySelectorAll("a")[1];
  const parts = premiumParts(premium);
  expect(parts).toHaveLength(2);
  expect(parts[0]).toEqual(["Premium", "tracking-[0.06em] text-[#E8C766]"]);
  expect(parts[1][1]).toBe("text-pure-white");
  expect(premium?.textContent).toBe(`Premium ${parts[1][0]}`);
});

it("プレミアム価格がないコーチはスタンダード1つだけを全幅で、現行の見た目のまま出す（本体 #1088 T5）", async () => {
  apiFetch.mockImplementation(async () => ({ ...coach, premium_price_yen: null }));
  await render();
  const buttons = [...(cta()?.querySelectorAll("a") ?? [])];
  expect(buttons).toHaveLength(1);
  expect(buttons[0].className).toBe(standardCtaClassName);
  expect(cta()?.innerHTML).not.toContain("#C9A227");
});

it("受付停止中のプレミアムは黒系背景と薄い金の枠で区別し、文字は灰色のまま色分けしない（本体 #1088 T6）", async () => {
  apiFetch.mockImplementation(async () => ({ ...coach, on_duty: false, premium_price_yen: 6000 }));
  await render();
  const [standard, premium] = [...(cta()?.querySelectorAll('[aria-disabled="true"]') ?? [])];
  expect([standard.tagName, premium.tagName]).toEqual(["SPAN", "SPAN"]);
  expect(standard.className).toBe("flex min-h-11 flex-1 cursor-not-allowed items-center justify-center rounded-md bg-white/10 px-3 text-sm font-semibold text-slate-400");
  expect(premium.textContent).toBe("プレミアム ¥6,000");
  expect(premium.children).toHaveLength(0);
  for (const cls of ["bg-[linear-gradient(135deg,#0D0D0D,#2B2B2B)]", "border", "border-[#C9A227]/40", "text-slate-400", "cursor-not-allowed"]) {
    expect(premium.className.split(" ")).toContain(cls);
  }
  expect(premium.className).not.toContain("bg-white/10");
});

it("受付停止中は、CTA を停止表示にし、依頼へ進めない", async () => {
  apiFetch.mockImplementation(async () => ({ ...coach, on_duty: false }));
  await render();
  expect(container.textContent).toContain("現在は受付を停止しています");
  expect(link("スタンダード")).toBeUndefined();
  expect(container.querySelector('[aria-disabled="true"]')?.textContent).toBe("スタンダード ¥1,000");
  // T3（本体 #1041）：停止中の span はリンクにせず、移動中表示の属性も付けない。
  const disabled = cta()?.querySelector('[aria-disabled="true"]');
  expect(disabled?.tagName).toBe("SPAN");
  expect(cta()?.querySelector("a")).toBeNull();
  expect(disabled?.hasAttribute("data-pending-link")).toBe(false);
  expect(disabled?.hasAttribute("data-screen-loader")).toBe(false);
  expect(cta()?.querySelector("[data-pending-link], [data-screen-loader]")).toBeNull();
});

const ctaHref = "/coaching/new?coach=2f6d0a8e-1111-4c2b-9a7e-3c332627b25f&pay=card";

it("固定 CTA の価格は特集コーチのいま有効な価格（API の応答）に追随する（本体 #1052 T10）", async () => {
  apiFetch.mockImplementation(async () => ({ ...coach, review_price_yen: 2500 }));
  await render();
  expect(cta()?.querySelector("a")?.textContent).toBe("スタンダード ¥2,500");
  expect(container.textContent).not.toContain("¥1,000");
});

it("受付中の固定 CTA は、href・色はそのままで、移動中表示の属性を持つ（本体 #1041 T1）", async () => {
  await render();
  const a = cta()?.querySelector("a");
  expect(a?.hasAttribute("data-pending-link")).toBe(true);
  expect(a?.hasAttribute("data-screen-loader")).toBe(true);
  expect(a?.getAttribute("href")).toBe(ctaHref);
  expect(a?.textContent).toBe("スタンダード ¥1,000");
  expect(a?.className).toBe("flex min-h-11 flex-1 items-center justify-center rounded-md bg-[#EF442D] px-3 text-sm font-semibold text-pure-white transition active:scale-[0.98]");
  expect(a?.hasAttribute("prefetch")).toBe(false);
});

it("受付中の固定 CTA をタップすると画面を開いている幕を出し、解除で消える（本体 #1041 T2）", async () => {
  await act(async () => root.render(<><FeaturedCoachView coachIDs={["2f6d0a8e-1111-4c2b-9a7e-3c332627b25f"]} /><ScreenLoader /></>));
  await act(async () => { await Promise.resolve(); });
  const loader = () => container.querySelector(".screen-loader");
  expect(loader()?.className).toContain("hidden");
  expect(container.querySelector('[role="status"]')).toBeNull();
  const onNavigate = navigateHandlers.get(ctaHref);
  expect(onNavigate).toBeTypeOf("function");
  await act(async () => onNavigate?.());
  expect(loader()?.className).toContain("flex");
  expect(loader()?.className).not.toContain("hidden");
  expect(container.querySelector('[role="status"]')?.textContent).toBe("画面を開いています");
  await act(async () => clearLinkPending());
  expect(loader()?.className).toContain("hidden");
  expect(container.querySelector('[role="status"]')).toBeNull();
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
  apiFetch.mockImplementation(async () => ({ ...coach, premium_price_yen: 6000 }));
  await render();
  // 英語表示の価格は固定レート ¥160=$1 のドルだけ（本体 #1085 T2）。¥1,000 → $6.25、¥6,000 → $37.50。
  expect(link("Standard $6.25")?.getAttribute("href")).toBe("/coaching/new?coach=2f6d0a8e-1111-4c2b-9a7e-3c332627b25f&pay=card");
  expect(link("Premium $37.50")?.getAttribute("href")).toBe(premiumHref);
  expect(cta()?.textContent).not.toContain("¥");
  expect(lead()?.textContent).toBe("Get your form and training diagnosed!");
  expect(container.textContent).toContain("Returned within 24 hours");
  expect(stepHeading(steps())).toBe("Standard");
  expect(stepHeading(premiumSteps())).toBe("Premium");
  expect(stepTexts(premiumSteps())).toEqual(["1Just send videos grouped by theme (up to 6)", "2The coach reviews them all and diagnoses", "3Answers and a summary returned within 24 hours"]);
  expect(container.textContent).not.toContain("FAQ");
});

it("英語表示の受付停止中も、固定 CTA の2つの span はドルで出し、リンクにしない（本体 #1085 T2）", async () => {
  installEnglish(en);
  setActiveLocale("en");
  apiFetch.mockImplementation(async () => ({ ...coach, on_duty: false, premium_price_yen: 6000 }));
  await render();
  const disabled = [...(cta()?.querySelectorAll('[aria-disabled="true"]') ?? [])];
  expect(disabled.map((el) => [el.tagName, el.textContent])).toEqual([["SPAN", "Standard $6.25"], ["SPAN", "Premium $37.50"]]);
  expect(cta()?.querySelector("a")).toBeNull();
  expect(cta()?.textContent).not.toContain("¥");
});

it("記録の欄を区切りでチップに分ける", () => {
  expect(recordChips("100m 10.76、400m 47.54\n200m 21.9")).toEqual(["100m 10.76", "400m 47.54", "200m 21.9"]);
  expect(recordChips(null)).toEqual([]);
});

// 特集コーチの横スライド（本体 #1063）。プロフィールだけを切り替え、流れの3ステップ（スタンダード・プレミアム）は1組。固定 CTA（価格）は表示中のコーチに連動する。
const second = {
  ...coach, id: "7c1e9b20-2222-4d3a-8b6f-5e4433221100", nickname: "テスト次郎", photo_url: "https://img.example/d.jpg",
  best_record: "200m 21.50", achievements: "大学の短距離コーチ", hitokoto: "一緒に走りましょう。", sns_urls: [], on_duty: false, review_price_yen: 1500,
};
const twoIDs = [coach.id, second.id];
const renderIDs = async (ids: string[]) => {
  await act(async () => root.render(<FeaturedCoachView coachIDs={ids} />));
  await act(async () => { await Promise.resolve(); });
};
const slides = () => [...container.querySelectorAll('[data-testid="featured-slide"]')];
const dots = () => [...container.querySelectorAll<HTMLButtonElement>('[data-testid="featured-dots"] button')];
const currentDot = () => dots().findIndex((dot) => dot.getAttribute("aria-current") === "true");
const visibleName = () => slides().find((slide) => slide.getAttribute("aria-hidden") !== "true")?.querySelector("h2")?.textContent;
const scroller = () => container.querySelector<HTMLDivElement>('[data-testid="featured-slides"] [role="region"]')!;
const swipeTo = async (index: number) => {
  const el = scroller();
  Object.defineProperty(el, "clientWidth", { configurable: true, value: 360 });
  el.scrollLeft = index * 360;
  await act(async () => { el.dispatchEvent(new Event("scroll")); });
};
const mockTwo = (secondResult: () => unknown) => apiFetch.mockImplementation(async (path: string) => {
  if (path === `/v1/coaches/${coach.id}`) return { ...coach };
  if (path === `/v1/coaches/${second.id}`) return secondResult();
  throw new Error(`unexpected ${path}`);
});

it("W1・W2・W6：特集2人では初期表示が1人目で、スワイプとドットで2人目に切り替わり、流れの3ステップは1組だけ", async () => {
  mockTwo(() => ({ ...second }));
  await renderIDs(twoIDs);
  expect(apiFetch.mock.calls.map(([path]) => path)).toEqual([`/v1/coaches/${coach.id}`, `/v1/coaches/${second.id}`]);
  expect(slides()).toHaveLength(2);
  expect(visibleName()).toBe("テスト太郎コーチについて");
  expect(dots()).toHaveLength(2);
  expect(dots().map((dot) => dot.getAttribute("aria-label"))).toEqual(["1人目のコーチを表示", "2人目のコーチを表示"]);
  expect(currentDot()).toBe(0);
  // 表示していないスライドは読み上げ・フォーカスから外す。
  expect(slides()[1].getAttribute("aria-hidden")).toBe("true");
  expect(slides()[1].hasAttribute("inert")).toBe(true);
  // 流れの3ステップ（スタンダード・プレミアム）は1組だけで、スライドの外（下）にある。よくある質問は無い（本体 #1054）。
  expect(container.querySelectorAll('[data-testid="featured-steps"]')).toHaveLength(1);
  expect(container.querySelectorAll('[data-testid="featured-premium-steps"]')).toHaveLength(1);
  expect(container.querySelectorAll('section[aria-labelledby="featured-faq"]')).toHaveLength(0);
  expect(scroller().contains(steps())).toBe(false);
  expect(scroller().contains(premiumSteps())).toBe(false);
  // ドットは写真（スライド）の上、3ステップはスライドの下。
  expect(before(container.querySelector('[data-testid="featured-dots"]'), scroller())).toBe(true);
  expect(before(container.querySelector('[data-testid="featured-dots"]'), slides()[0].querySelector('span.rounded-full, img'))).toBe(true);
  expect(before(scroller(), steps())).toBe(true);
  // スワイプ（スクロールスナップ）で2人目へ。
  await swipeTo(1);
  expect(visibleName()).toBe("テスト次郎コーチについて");
  expect(currentDot()).toBe(1);
  // ドットのタップで1人目へ戻る。
  await act(async () => dots()[0].click());
  expect(visibleName()).toBe("テスト太郎コーチについて");
  expect(currentDot()).toBe(0);
  await act(async () => dots()[1].click());
  expect(currentDot()).toBe(1);
  // スライドで取り直さない。
  expect(apiFetch).toHaveBeenCalledTimes(2);
});

it("W3：固定 CTA の依頼リンク・価格と受付状態は表示中のコーチに従う", async () => {
  mockTwo(() => ({ ...second }));
  await renderIDs(twoIDs);
  expect(link("スタンダード ¥1,000")?.getAttribute("href")).toBe(`/coaching/new?coach=${coach.id}&pay=card`);
  expect(container.textContent).not.toContain("現在は受付を停止しています");
  await act(async () => dots()[1].click());
  // 2人目は受付停止中。価格は2人目のいま有効な価格（本体 #1052）。
  expect(link("スタンダード")).toBeUndefined();
  expect(cta()?.textContent).toContain("現在は受付を停止しています");
  expect(cta()?.querySelector('[aria-disabled="true"]')?.textContent).toBe("スタンダード ¥1,500");
  mockTwo(() => ({ ...second, on_duty: true }));
  await act(async () => root.unmount());
  root = createRoot(container);
  await renderIDs(twoIDs);
  await swipeTo(1);
  expect(link("スタンダード ¥1,500")?.getAttribute("href")).toBe(`/coaching/new?coach=${second.id}&pay=card`);
});

it("W4：特集1人ではスライドもドットも出さない（従来の表示と同じ）", async () => {
  await render();
  expect(container.querySelector('[data-testid="featured-slides"]')).toBeNull();
  expect(container.querySelector('[data-testid="featured-dots"]')).toBeNull();
  expect(container.querySelector('[role="region"]')).toBeNull();
  expect(about()?.querySelector("h2")?.textContent).toBe("テスト太郎コーチについて");
});

it("W5：2人目が非公開（404）ならスライドから外し、1人だけの表示にする（エラーにしない）", async () => {
  const { ApiRequestError } = await import("@/lib/api");
  mockTwo(() => { throw new ApiRequestError(404, { error: { code: "coach_not_public", message: "x" } }); });
  await renderIDs(twoIDs);
  expect(container.querySelector('[role="alert"]')).toBeNull();
  expect(container.querySelector('[data-testid="featured-slides"]')).toBeNull();
  expect(container.querySelector('[data-testid="featured-dots"]')).toBeNull();
  expect(about()?.querySelector("h2")?.textContent).toBe("テスト太郎コーチについて");
  expect(link("スタンダード ¥1,000")?.getAttribute("href")).toBe(`/coaching/new?coach=${coach.id}&pay=card`);
});

it("1人目が非公開なら、取得できた2人目を先頭にする。通信エラーは従来のエラー表示", async () => {
  const { ApiRequestError } = await import("@/lib/api");
  apiFetch.mockImplementation(async (path: string) => {
    if (path === `/v1/coaches/${coach.id}`) throw new ApiRequestError(404, { error: { code: "coach_not_public", message: "x" } });
    return { ...second };
  });
  await renderIDs(twoIDs);
  expect(about()?.querySelector("h2")?.textContent).toBe("テスト次郎コーチについて");
  await act(async () => root.unmount());
  root = createRoot(container);
  mockTwo(() => { throw new Error("network"); });
  await renderIDs(twoIDs);
  expect(container.querySelector('[role="alert"]')?.textContent).toBe("読み込みに失敗しました");
});

it("W6：2人目を見たあとに開き直しても先頭から出し、表示位置を保存しない", async () => {
  const setItem = vi.spyOn(Storage.prototype, "setItem");
  mockTwo(() => ({ ...second }));
  await renderIDs(twoIDs);
  await act(async () => dots()[1].click());
  expect(currentDot()).toBe(1);
  await act(async () => root.unmount());
  root = createRoot(container);
  await renderIDs(twoIDs);
  expect(currentDot()).toBe(0);
  expect(visibleName()).toBe("テスト太郎コーチについて");
  expect(setItem).not.toHaveBeenCalled();
  setItem.mockRestore();
});

it("W8：スライドの文言は英語辞書にあり、英語表示で英語になる", async () => {
  installEnglish(en);
  setActiveLocale("en");
  mockTwo(() => ({ ...second }));
  await renderIDs(twoIDs);
  expect(scroller().getAttribute("aria-label")).toBe(en["コーチのプロフィール"]);
  expect(dots().map((dot) => dot.getAttribute("aria-label"))).toEqual(["Show coach 1", "Show coach 2"]);
});
