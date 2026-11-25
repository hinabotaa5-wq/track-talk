/** @vitest-environment jsdom */
import { act, type ReactNode } from "react";
import { createRoot, type Root } from "react-dom/client";
import { afterEach, beforeEach, expect, it, vi } from "vitest";
import { installEnglish, setActiveLocale } from "@/lib/i18n";
import { en } from "@/lib/i18n-en";

// /coaching と /coaches で共有する一覧の切り替え。共通部品 ViewSwitch を使い、見出しと説明文は出さない。
// 継続プランは休眠させたので（本体 #1007）、選択肢はスタンダードプランだけで、継続プランの一覧は描画しない。
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

it("既定はスタンダードプランの一覧で、見出しと説明文は出さない", async () => {
  await render();
  expect(panels()).toHaveLength(1);
  expect(fetchCount("/v1/coaches")).toBe(1);
  expect(fetchCount("/v1/coaches/support-plan")).toBe(0);
  expect(container.querySelector("h1")).toBeNull();
  expect(container.textContent).not.toContain("スタンダードプラン（フォーム添削や競技に関する悩み）");
  expect(container.textContent).not.toContain("フォームを見てほしい場合は動画を添えてください。悩みだけでも申し込めます。");
  expect(container.textContent).not.toContain("コーチ一覧");
  expect(switchLink("スタンダードプラン")?.getAttribute("href")).toBe("/coaches");
  expect(switchLink("スタンダードプラン")?.getAttribute("aria-current")).toBe("page");
});

// 本体 #1007 B1：継続プランは休眠。切り替えに出るのはスタンダードプランだけで、SupportPlanList は描画しない。
for (const supportList of [true, false]) {
  it(`切り替えの選択肢はスタンダードプランだけで、?tab=support でもスタンダードプラン側を出す（一覧フラグ=${supportList}）`, async () => {
    flags.supportList = supportList;
    navigation.tab = "support";
    await render();
    const options = [...container.querySelectorAll('[role="group"] a')].map((a) => a.textContent);
    expect(options).toEqual(["スタンダードプラン"]);
    expect(switchLink("継続プラン")).toBeUndefined();
    expect(switchLink("スタンダードプラン")?.getAttribute("aria-current")).toBe("page");
    expect(panels()).toHaveLength(1);
    expect(panels()[0].hidden).toBe(false);
    expect(fetchCount("/v1/coaches")).toBe(1);
    expect(fetchCount("/v1/coaches/support-plan")).toBe(0);
    expect(container.textContent).not.toContain("継続プラン");
    expect(container.querySelector('[role="radio"]')).toBeNull();
  });
}

it("英語でも見出しを出さず、切り替えの選択肢は英語になる", async () => {
  installEnglish(en);
  setActiveLocale("en");
  await render();
  expect(switchLink("Standard Plan")?.getAttribute("href")).toBe("/coaches");
  expect(switchLink("Ongoing plan")).toBeUndefined();
  expect(container.textContent).not.toContain("Coaches");
  expect(container.textContent).not.toContain("Standard Plan (form feedback or concerns about competing)");
  expect(container.textContent).not.toContain("Attach a video if you would like form feedback. You can also ask for advice without a video.");
});

for (const pathname of ["/coaching", "/coaches"]) {
  it(`${pathname} では切り替えのリンク先が ${pathname} のままで、押しても画面遷移しない`, async () => {
    navigation.pathname = pathname;
    await render();
    expect(switchLink("スタンダードプラン")?.getAttribute("href")).toBe(pathname);
    await act(async () => switchLink("スタンダードプラン")!.click());
    expect(replaceState).not.toHaveBeenCalled();
    expect(navigation.replace).not.toHaveBeenCalled();
    expect(navigation.push).not.toHaveBeenCalled();
    expect(fetchCount("/v1/coaches")).toBe(1);
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
  expect(switchLink("スタンダードプラン")).not.toBeUndefined();
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
  expect(switchLink("スタンダードプラン")).toBeUndefined();
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
