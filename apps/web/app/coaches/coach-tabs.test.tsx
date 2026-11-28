/** @vitest-environment jsdom */
import { act, type ReactNode } from "react";
import { createRoot, type Root } from "react-dom/client";
import { afterEach, beforeEach, expect, it, vi } from "vitest";
import { installEnglish, setActiveLocale } from "@/lib/i18n";
import { en } from "@/lib/i18n-en";

// /coaching と /coaches で共有する一覧の切り替え。共通部品 ViewSwitch を使い、見出しと説明文は出さない。
// 切り替えはスタンダード／プレミアムの2択（本体 #1026）。プレミアム側は、プレミアム価格があるコーチだけの一覧（本体 #1053）。
// 継続プランは休眠させたので（本体 #1007）、継続プランの選択肢と一覧は描画しない。
const apiFetch = vi.fn();
const navigation = vi.hoisted(() => ({ replace: vi.fn(), push: vi.fn(), tab: null as string | null, pathname: "/coaches" }));
const auth = { token: "tok" as string | null };
const flags = { supportList: true };
vi.mock("next/navigation", () => ({
  useRouter: () => ({ replace: navigation.replace, push: navigation.push }),
  useSearchParams: () => ({ get: (key: string) => (key === "tab" ? navigation.tab : null) }),
  usePathname: () => navigation.pathname,
}));
vi.mock("next/link", () => ({ default: ({ href, children, ...props }: { href: string; children: ReactNode }) => <a href={href} {...props}>{children}</a> }));
vi.mock("@/lib/api", async (original) => ({ ...await original<typeof import("@/lib/api")>(), apiFetch: (...args: unknown[]) => apiFetch(...args) }));
vi.mock("@/lib/features", async (original) => ({ ...await original<typeof import("@/lib/features")>(), get supportPlanListEnabled() { return flags.supportList; } }));
vi.mock("@/components/auth-provider", () => ({ useAuth: () => ({ token: auth.token, loading: false }) }));
const { CoachTabs } = await import("./coach-tabs");

let root: Root;
let container: HTMLDivElement;
let replaceState: ReturnType<typeof vi.spyOn>;
const fetchCount = (path: string) => apiFetch.mock.calls.filter(([p]) => p === path).length;
beforeEach(() => {
  (globalThis as typeof globalThis & { IS_REACT_ACT_ENVIRONMENT: boolean }).IS_REACT_ACT_ENVIRONMENT = true;
  container = document.createElement("div"); document.body.appendChild(container); root = createRoot(container);
  apiFetch.mockReset(); navigation.replace.mockReset(); navigation.push.mockReset();
  navigation.tab = null; navigation.pathname = "/coaches";
  auth.token = "tok"; flags.supportList = true;
  apiFetch.mockImplementation(async (path: string) => {
    if (path === "/v1/coaches") return { coaches: [] };
    if (path === "/v1/coaches/support-plan") return { coaches: [] };
    throw new Error(`unexpected ${path}`);
  });
  replaceState = vi.spyOn(window.history, "replaceState");
  window.history.replaceState(null, "", "/");
  replaceState.mockClear();
});
afterEach(async () => { await act(async () => root.unmount()); container.remove(); replaceState.mockRestore(); setActiveLocale("ja"); });
const render = async () => {
  await act(async () => root.render(<CoachTabs />));
  await act(async () => { await Promise.resolve(); await Promise.resolve(); });
};
const switchLink = (label: string) => [...container.querySelectorAll("a")].find((a) => a.textContent === label);
// tabpanel 直下のパネル（開いた順）。hidden の有無で表示・退避を見分ける。
const panels = () => [...container.querySelectorAll('[role="tabpanel"] > div')] as HTMLElement[];
const coachButton = () => [...container.querySelectorAll('a[href="/coach"]')].find((a) => a.getAttribute("aria-label") === "コーチ");
const playerButton = () => [...container.querySelectorAll('a[href="/player"]')].find((a) => a.getAttribute("aria-label") === "選手");

const options = () => [...container.querySelectorAll('[role="group"] a')].map((a) => a.textContent);
const premiumText = "プレミアムは準備中です";
// プレミアムタブの一覧（本体 #1053）。価格のあるコーチだけを出す。
const coach = (id: string, nickname: string, premium: number | null, onDuty = true) => ({
  id, nickname, photo_url: "", tier: "coach", events: ["100m"], best_record: "", achievements: "", years: 1, sns_urls: [], hitokoto: "",
  rating_count: 0, rating_average: 0, on_duty: onDuty, country_code: "JP", plans: [], service_types: [], premium_price_yen: premium,
});
const premiumCoaches = [coach("c-priced", "価格ありコーチ", 8000), coach("c-plain", "価格なしコーチ", null), coach("c-paused", "停止中コーチ", 12000, false)];
const premiumList = () => container.querySelector('[data-testid="premium-coach-list"]');

// T1
it("既定はスタンダードの一覧で、切り替えはスタンダード／プレミアムの2択。見出しと説明文は出さない", async () => {
  await render();
  expect(options()).toEqual(["スタンダード", "プレミアム"]);
  expect(switchLink("プレミアム")?.getAttribute("aria-current")).toBeNull();
  expect(container.textContent).not.toContain(premiumText);
  expect(premiumList()).toBeNull();
  expect(panels()).toHaveLength(1);
  expect(fetchCount("/v1/coaches")).toBe(1);
  expect(fetchCount("/v1/coaches/support-plan")).toBe(0);
  expect(container.querySelector("h1")).toBeNull();
  expect(container.textContent).not.toContain("スタンダード（フォーム添削や競技に関する悩み）");
  expect(container.textContent).not.toContain("フォームを見てほしい場合は動画を添えてください。悩みだけでも申し込めます。");
  expect(container.textContent).not.toContain("コーチ一覧");
  expect(switchLink("スタンダード")?.getAttribute("href")).toBe("/coaches");
  expect(switchLink("スタンダード")?.getAttribute("aria-current")).toBe("page");
});

// T2（本体 #1053 で書き換え）：プレミアム側は、プレミアム価格があるコーチだけの一覧。価格は API の値を出し、受付中のコーチはプレミアムの申込みへ導く。
it("「プレミアム」を押すとその場でプレミアムのコーチ一覧に切り替わり、価格のあるコーチだけを出す", async () => {
  apiFetch.mockImplementation(async (path: string) => {
    if (path === "/v1/coaches") return { coaches: premiumCoaches };
    throw new Error(`unexpected ${path}`);
  });
  await render();
  await act(async () => switchLink("プレミアム")!.click());
  await act(async () => { await Promise.resolve(); await Promise.resolve(); });
  expect(replaceState).toHaveBeenLastCalledWith(null, "", "/coaches?tab=premium");
  expect(navigation.replace).not.toHaveBeenCalled();
  expect(navigation.push).not.toHaveBeenCalled();
  expect(switchLink("プレミアム")?.getAttribute("aria-current")).toBe("page");
  const visible = panels().filter((panel) => !panel.hidden);
  expect(visible).toHaveLength(1);
  expect(visible[0].textContent).not.toContain(premiumText);
  const list = premiumList()!;
  expect(list.textContent).toContain("価格ありコーチ");
  expect(list.textContent).toContain("8,000円");
  expect(list.textContent).not.toContain("価格なしコーチ");
  // 受付停止中のコーチは一覧に残すが、申込みの導線は出さない。
  expect(list.textContent).toContain("停止中コーチ");
  expect(list.textContent).toContain("受付停止中");
  const links = [...list.querySelectorAll("a")].map((a) => a.getAttribute("href"));
  expect(links).toEqual(["/coaching/new?plan=premium&coach=c-priced&pay=card"]);
  expect(fetchCount("/v1/coaches/support-plan")).toBe(0);
});

// 本体 #1085：英語表示ではプレミアム一覧の価格もドルだけ（¥160=$1）で出す。¥8,000 → $50.00、¥12,000 → $75.00。
it("英語表示ではプレミアム一覧の価格をドルで出す（#1085）", async () => {
  installEnglish(en);
  setActiveLocale("en");
  apiFetch.mockImplementation(async (path: string) => {
    if (path === "/v1/coaches") return { coaches: premiumCoaches };
    throw new Error(`unexpected ${path}`);
  });
  await render();
  await act(async () => switchLink("Premium")!.click());
  await act(async () => { await Promise.resolve(); await Promise.resolve(); });
  const list = premiumList()!;
  expect(list.textContent).toContain("$50.00");
  expect(list.textContent).toContain("$75.00");
  expect(list.textContent).not.toMatch(/¥|円/);
});

// T3
it("プレミアムからスタンダードに戻すと URL が base に戻り、コーチ一覧は取り直さない", async () => {
  await render();
  await act(async () => switchLink("プレミアム")!.click());
  await act(async () => switchLink("スタンダード")!.click());
  expect(replaceState).toHaveBeenLastCalledWith(null, "", "/coaches");
  expect(switchLink("スタンダード")?.getAttribute("aria-current")).toBe("page");
  const visible = panels().filter((panel) => !panel.hidden);
  expect(visible).toHaveLength(1);
  expect(visible[0].textContent).not.toContain(premiumText);
  expect(container.querySelector(".page-loading-screen")).toBeNull();
  // スタンダードの一覧とプレミアムの一覧で1回ずつ。行き来しても取り直さない。
  expect(fetchCount("/v1/coaches")).toBe(2);
});

// T4
it("?tab=premium で開くとプレミアム側から出し、スタンダードに切り替えて初めてスタンダードの一覧を取得する", async () => {
  navigation.tab = "premium";
  await render();
  expect(container.querySelector(".page-loading-screen")).toBeNull();
  expect(options()).toEqual(["スタンダード", "プレミアム"]);
  expect(switchLink("プレミアム")?.getAttribute("aria-current")).toBe("page");
  expect(panels()).toHaveLength(1);
  expect(panels()[0].textContent).not.toContain(premiumText);
  expect(coachButton()?.getAttribute("href")).toBe("/coach");
  expect(fetchCount("/v1/coaches")).toBe(1);
  await act(async () => switchLink("スタンダード")!.click());
  await act(async () => { await Promise.resolve(); await Promise.resolve(); });
  expect(fetchCount("/v1/coaches")).toBe(2);
  expect(replaceState).toHaveBeenLastCalledWith(null, "", "/coaches");
  expect(switchLink("スタンダード")?.getAttribute("aria-current")).toBe("page");
  expect(panels()).toHaveLength(2);
});

// T5（本体 #1007 B1 を書き換え）：継続プランは休眠。?tab=support でもスタンダード側を出し、SupportPlanList は描画しない。
for (const supportList of [true, false]) {
  it(`?tab=support で開いてもスタンダード側を出し、継続プランの選択肢・文言は出さない（一覧フラグ=${supportList}）`, async () => {
    flags.supportList = supportList;
    navigation.tab = "support";
    await render();
    expect(options()).toEqual(["スタンダード", "プレミアム"]);
    expect(switchLink("継続プラン")).toBeUndefined();
    expect(switchLink("スタンダード")?.getAttribute("aria-current")).toBe("page");
    expect(panels()).toHaveLength(1);
    expect(panels()[0].hidden).toBe(false);
    expect(fetchCount("/v1/coaches")).toBe(1);
    expect(fetchCount("/v1/coaches/support-plan")).toBe(0);
    expect(container.textContent).not.toContain("継続プラン");
    expect(container.querySelector('[role="radio"]')).toBeNull();
  });
}

// T7
it("英語でも見出しを出さず、切り替えの選択肢は Standard／Premium、プレミアム側の文も英語になる", async () => {
  installEnglish(en);
  setActiveLocale("en");
  await render();
  expect(options()).toEqual(["Standard", "Premium"]);
  expect(switchLink("Standard")?.getAttribute("href")).toBe("/coaches");
  expect(switchLink("Premium")?.getAttribute("href")).toBe("/coaches?tab=premium");
  await act(async () => switchLink("Premium")!.click());
  await act(async () => { await Promise.resolve(); await Promise.resolve(); });
  const premiumPanel = panels().filter((panel) => !panel.hidden)[0].textContent ?? "";
  expect(premiumPanel).toContain("Send up to 6 videos in total, grouped by theme");
  expect(premiumPanel).toContain("Race, Start, Second half, Practice, Other");
  expect(premiumPanel).toMatch(/^[\x00-\x7F]*$/);
  expect(switchLink("Ongoing plan")).toBeUndefined();
  expect(container.textContent).not.toContain("Coaches");
  expect(container.textContent).not.toContain("Standard (form feedback or concerns about competing)");
  expect(container.textContent).not.toContain("Attach a video if you would like form feedback. You can also ask for advice without a video.");
});

// T6
for (const pathname of ["/coaching", "/coaches"]) {
  it(`${pathname} では切り替えのリンク先が ${pathname} のままで、押しても画面遷移しない`, async () => {
    navigation.pathname = pathname;
    await render();
    expect(switchLink("スタンダード")?.getAttribute("href")).toBe(pathname);
    expect(switchLink("プレミアム")?.getAttribute("href")).toBe(`${pathname}?tab=premium`);
    await act(async () => switchLink("スタンダード")!.click());
    expect(replaceState).not.toHaveBeenCalled();
    await act(async () => switchLink("プレミアム")!.click());
    expect(replaceState).toHaveBeenLastCalledWith(null, "", `${pathname}?tab=premium`);
    expect(navigation.replace).not.toHaveBeenCalled();
    expect(navigation.push).not.toHaveBeenCalled();
    expect(fetchCount("/v1/coaches")).toBe(2);
  });
}

it("狭い画面でも切り替えと円ボタン2つは同じ行に収まる", async () => {
  await render();
  const row = container.querySelector('[role="group"]')?.closest("div.flex");
  expect(row).not.toBeNull();
  expect(row?.contains(playerButton()!)).toBe(true);
  expect(row?.contains(coachButton()!)).toBe(true);
});

// 運営者決定（2026-11-14／2026-11-17）：ログイン状態・応募有無によらず常時出す。入口はアイコンだけの円ボタン2つ。
it("円ボタンは左が選手（/player）・右がコーチ（/coach）で、文字ラベルは無い", async () => {
  await render();
  expect(playerButton()?.getAttribute("href")).toBe("/player");
  expect(playerButton()?.getAttribute("aria-label")).toBe("選手");
  expect(coachButton()?.getAttribute("href")).toBe("/coach");
  expect(coachButton()?.getAttribute("aria-label")).toBe("コーチ");
  expect(playerButton()?.textContent).toBe("");
  expect(coachButton()?.textContent).toBe("");
  const row = container.querySelector('[role="group"]')?.closest("div.flex");
  const links = [...(row?.querySelectorAll("a") ?? [])] as HTMLAnchorElement[];
  expect(links.indexOf(playerButton() as HTMLAnchorElement)).toBeLessThan(links.indexOf(coachButton() as HTMLAnchorElement));
});

it("英語では円ボタンの aria-label が Athlete／Coach になる", async () => {
  installEnglish(en);
  setActiveLocale("en");
  await render();
  expect([...container.querySelectorAll('a[href="/player"]')].find((a) => a.getAttribute("aria-label") === "Athlete")).not.toBeUndefined();
  expect([...container.querySelectorAll('a[href="/coach"]')].find((a) => a.getAttribute("aria-label") === "Coach")).not.toBeUndefined();
});

it("未ログインでも円ボタン2つを出し、/player・/coach へ導く（応募状態を取得しない）", async () => {
  auth.token = null;
  await render();
  expect(playerButton()?.getAttribute("href")).toBe("/player");
  expect(coachButton()?.getAttribute("href")).toBe("/coach");
  expect(apiFetch.mock.calls.some(([path]) => path === "/v1/coach/me")).toBe(false);
});

it("ログイン済み・応募なしでも円ボタン2つを出す（応募状態を取得しない）", async () => {
  await render();
  expect(playerButton()?.getAttribute("href")).toBe("/player");
  expect(coachButton()?.getAttribute("href")).toBe("/coach");
  expect(apiFetch.mock.calls.some(([path]) => path === "/v1/coach/me")).toBe(false);
});

// 本体 #804：エラー時は読み込み中扱いを終え、従来のエラー表示に到達する。ストップウォッチだけが残り続けない。
it("取得に失敗したらストップウォッチを消してエラーを出し、切り替えを戻す", async () => {
  apiFetch.mockRejectedValue(new Error("network down"));
  await render();
  expect(container.querySelector(".page-loading-screen")).toBeNull();
  expect(container.querySelector('[role="alert"]')?.textContent).toBe("読み込みに失敗しました");
  expect(switchLink("スタンダード")).not.toBeUndefined();
  expect(coachButton()?.getAttribute("href")).toBe("/coach");
  expect(fetchCount("/v1/coaches")).toBe(1);
});

it.each(["pending", "approved", "rejected", "suspended"])("応募あり（status=%s）でも円ボタン2つを出す（応募状態を取得しない）", async (status) => {
  apiFetch.mockImplementation(async (path: string) => {
    if (path === "/v1/coach/me") return { application: { status } };
    if (path === "/v1/coaches") return { coaches: [] };
    if (path === "/v1/coaches/support-plan") return { coaches: [] };
    throw new Error(`unexpected ${path}`);
  });
  await render();
  expect(coachButton()?.getAttribute("href")).toBe("/coach");
  expect(apiFetch.mock.calls.some(([path]) => path === "/v1/coach/me")).toBe(false);
});

// 本体 #804：初回読み込み中はページ中身をストップウォッチだけにする。
// 取得が終わるまで切り替え・コーチボタン・種目チップは DOM に出さない。
it("初回の取得が終わるまで切り替えとコーチボタンを出さず、ストップウォッチだけを出す", async () => {
  apiFetch.mockReturnValue(new Promise(() => {}));
  await render();
  expect(container.querySelector(".page-loading-screen")).not.toBeNull();
  expect(container.querySelector('[role="status"]')).not.toBeNull();
  expect(switchLink("スタンダード")).toBeUndefined();
  expect(coachButton()).toBeUndefined();
  expect(playerButton()).toBeUndefined();
  expect(container.querySelector('[role="radio"]')).toBeNull();
  // 応募状態は取得しない（従来どおり）。
  expect(apiFetch.mock.calls.some(([path]) => path === "/v1/coach/me")).toBe(false);
});

// 本体 #891：円ボタンの右上に、自分が対応しないと進まない用事の件数をバッジで出す。
const withCounts = (player: number, coach: number) => {
  apiFetch.mockImplementation(async (path: string) => {
    if (path === "/v1/me/coaching/action-counts") return { player_count: player, coach_count: coach };
    if (path === "/v1/coaches") return { coaches: [] };
    if (path === "/v1/coaches/support-plan") return { coaches: [] };
    throw new Error(`unexpected ${path}`);
  });
};
const anchorByHref = (href: string) => container.querySelector(`a[href="${href}"]`) as HTMLAnchorElement | null;
const badgeOf = (href: string) => anchorByHref(href)?.querySelector("span") ?? null;

it("件数があるときだけ、円ボタンに赤バッジと件数つきの aria-label を出す", async () => {
  withCounts(2, 3);
  await render();
  expect(fetchCount("/v1/me/coaching/action-counts")).toBe(1);
  expect(badgeOf("/player")?.textContent).toBe("2");
  expect(badgeOf("/coach")?.textContent).toBe("3");
  expect(anchorByHref("/player")?.getAttribute("aria-label")).toBe("選手（未対応2件）");
  expect(anchorByHref("/coach")?.getAttribute("aria-label")).toBe("コーチ（未対応3件）");
  // ベルのバッジと同じ見た目・円ボタンの大きさは変えない。
  const badge = badgeOf("/player")!;
  for (const cls of ["absolute", "rounded-full", "bg-red-600", "text-pure-white", "text-xs", "font-bold"]) expect(badge.classList.contains(cls)).toBe(true);
  expect(anchorByHref("/player")?.classList.contains("size-11")).toBe(true);
  expect(anchorByHref("/player")?.getAttribute("href")).toBe("/player");
  expect(anchorByHref("/coach")?.getAttribute("href")).toBe("/coach");
});

it("片方だけ件数があるときは、その円ボタンだけにバッジが付く", async () => {
  withCounts(0, 5);
  await render();
  expect(badgeOf("/player")).toBeNull();
  expect(badgeOf("/coach")?.textContent).toBe("5");
  expect(anchorByHref("/player")?.getAttribute("aria-label")).toBe("選手");
});

it("件数が両方 0 のときはバッジを出さず、aria-label は従来どおり", async () => {
  withCounts(0, 0);
  await render();
  expect(container.querySelectorAll('a[href="/player"] span, a[href="/coach"] span')).toHaveLength(0);
  expect(playerButton()?.textContent).toBe("");
  expect(coachButton()?.textContent).toBe("");
  expect(anchorByHref("/player")?.getAttribute("aria-label")).toBe("選手");
  expect(anchorByHref("/coach")?.getAttribute("aria-label")).toBe("コーチ");
});

it("未ログインでは集計 API を呼ばず、バッジなしで円ボタン2つを出す", async () => {
  auth.token = null;
  withCounts(2, 3);
  await render();
  expect(fetchCount("/v1/me/coaching/action-counts")).toBe(0);
  expect(container.querySelectorAll('a[href="/player"] span, a[href="/coach"] span')).toHaveLength(0);
  expect(playerButton()?.getAttribute("href")).toBe("/player");
  expect(coachButton()?.getAttribute("href")).toBe("/coach");
});

it("集計 API が失敗したら、バッジもエラー表示も出さず、円ボタン2つとリンク先は従来どおり", async () => {
  apiFetch.mockImplementation(async (path: string) => {
    if (path === "/v1/me/coaching/action-counts") throw new Error("boom");
    if (path === "/v1/coaches") return { coaches: [] };
    throw new Error(`unexpected ${path}`);
  });
  await render();
  expect(fetchCount("/v1/me/coaching/action-counts")).toBe(1);
  expect(container.querySelector('[role="alert"]')).toBeNull();
  expect(container.querySelectorAll('a[href="/player"] span, a[href="/coach"] span')).toHaveLength(0);
  expect(playerButton()?.getAttribute("href")).toBe("/player");
  expect(coachButton()?.getAttribute("href")).toBe("/coach");
});

it("件数が 100 以上のときバッジは「99+」になる（99 まではそのまま）", async () => {
  withCounts(100, 99);
  await render();
  expect(badgeOf("/player")?.textContent).toBe("99+");
  expect(badgeOf("/coach")?.textContent).toBe("99");
  expect(anchorByHref("/player")?.getAttribute("aria-label")).toBe("選手（未対応100件）");
});

it("英語では件数つき aria-label が Athlete (2 pending)／Coach (3 pending) になる", async () => {
  installEnglish(en);
  setActiveLocale("en");
  withCounts(2, 3);
  await render();
  expect(anchorByHref("/player")?.getAttribute("aria-label")).toBe("Athlete (2 pending)");
  expect(anchorByHref("/coach")?.getAttribute("aria-label")).toBe("Coach (3 pending)");
  expect(en["選手（未対応{count}件）"]).toBe("Athlete ({count} pending)");
  expect(en["コーチ（未対応{count}件）"]).toBe("Coach ({count} pending)");
});

it("タブに戻ったとき（visibilitychange で visible）とフォーカス時に件数を取り直す", async () => {
  withCounts(1, 0);
  await render();
  expect(fetchCount("/v1/me/coaching/action-counts")).toBe(1);
  withCounts(4, 0);
  await act(async () => { document.dispatchEvent(new Event("visibilitychange")); await Promise.resolve(); await Promise.resolve(); });
  expect(fetchCount("/v1/me/coaching/action-counts")).toBe(2);
  expect(badgeOf("/player")?.textContent).toBe("4");
  await act(async () => { window.dispatchEvent(new Event("focus")); await Promise.resolve(); await Promise.resolve(); });
  expect(fetchCount("/v1/me/coaching/action-counts")).toBe(3);
});

it("初回取得が終わるまで上部行（円ボタンとバッジ）を出さない", async () => {
  withCounts(2, 3);
  apiFetch.mockImplementation(() => new Promise(() => {}));
  await render();
  expect(container.querySelector('a[href="/player"]')).toBeNull();
  expect(container.querySelector('a[href="/coach"]')).toBeNull();
});
