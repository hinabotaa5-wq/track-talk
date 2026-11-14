/** @vitest-environment jsdom */
import { act, type ReactNode } from "react";
import { createRoot, type Root } from "react-dom/client";
import { afterEach, beforeEach, expect, it, vi } from "vitest";
import { installEnglish, setActiveLocale } from "@/lib/i18n";
import { en } from "@/lib/i18n-en";

// /coaching と /coaches で共有する一覧の切り替え。独自タブではなく共通部品 ViewSwitch を使い、
// 今いるルートのまま tab だけを変える。見出しと説明文は出さない。
// 大会・ケガのタブと同じく、その場の local state で即座に切り替え、URL は書き換えるだけ
// （router.replace／router.push は使わない）。一度開いた一覧は隠して残し、
// 行き来しても読み込み直さない。
const apiFetch = vi.fn();
const navigation = vi.hoisted(() => ({ replace: vi.fn(), push: vi.fn(), tab: null as string | null, pathname: "/coaches" }));
const auth = { token: "tok" as string | null };
const flags = { supportList: true };
vi.mock("next/navigation", () => ({
  useRouter: () => ({ replace: navigation.replace, push: navigation.push }),
  useSearchParams: () => ({ get: (key: string) => (key === "tab" ? navigation.tab : null) }),
  usePathname: () => navigation.pathname,
}));
vi.mock("next/link", () => ({ default: ({ href, children }: { href: string; children: ReactNode }) => <a href={href}>{children}</a> }));
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
const coachButton = () => [...container.querySelectorAll('a[href="/coach"]')].find((a) => a.textContent === "コーチ");
const chip = (label: string, scope: ParentNode = container) =>
  [...scope.querySelectorAll('[role="radio"]')].find((el) => el.textContent === label) as HTMLElement | undefined;

it("既定はフォーム添削の一覧で、見出しと説明文を出さない", async () => {
  await render();
  // フォーム添削側だけをマウントし、継続プラン側はまだ開いていない。
  expect(panels()).toHaveLength(1);
  expect(fetchCount("/v1/coaches")).toBe(1);
  expect(fetchCount("/v1/coaches/support-plan")).toBe(0);
  expect(container.querySelector("h1")).toBeNull();
  expect(container.textContent).not.toContain("コーチ一覧");
  expect(container.textContent).not.toContain("上のタブで、フォーム添削の指名と継続プランを切り替えられます。");
  // 切り替えは共通部品 ViewSwitch（2つの選択肢・専用の layoutId）。
  expect(switchLink("フォーム添削")?.getAttribute("href")).toBe("/coaches");
  expect(switchLink("継続プラン")?.getAttribute("href")).toBe("/coaches?tab=support");
});

it("?tab=support で直接開くと継続プラン側を出す", async () => {
  navigation.tab = "support";
  await render();
  expect(panels()).toHaveLength(1);
  expect(fetchCount("/v1/coaches/support-plan")).toBe(1);
  expect(fetchCount("/v1/coaches")).toBe(0);
  expect(switchLink("継続プラン")?.getAttribute("aria-current")).toBe("page");
  expect(container.querySelector("h1")).toBeNull();
  expect(container.textContent).not.toContain("コーチ一覧");
});

it("英語でも見出しを出さず、切り替えの選択肢は英語になる", async () => {
  installEnglish(en);
  setActiveLocale("en");
  await render();
  expect(switchLink("Form review")?.getAttribute("href")).toBe("/coaches");
  expect(switchLink("Ongoing plan")?.getAttribute("href")).toBe("/coaches?tab=support");
  expect(container.textContent).not.toContain("Coaches");
});

for (const pathname of ["/coaching", "/coaches"]) {
  it(`${pathname} では切り替え後も ${pathname} に残り、別ルートへ飛ばない`, async () => {
    navigation.pathname = pathname;
    await render();
    await act(async () => switchLink("継続プラン")!.click());
    await act(async () => { await Promise.resolve(); await Promise.resolve(); });
    // router による画面遷移は使わない。URL だけを書き換えてその場で切り替える。
    expect(navigation.replace).not.toHaveBeenCalled();
    expect(navigation.push).not.toHaveBeenCalled();
    expect(replaceState).toHaveBeenCalledExactlyOnceWith(null, "", `${pathname}?tab=support`);
    expect(panels()).toHaveLength(2);
    expect(panels()[0].hidden).toBe(true);
    expect(panels()[1].hidden).toBe(false);
    await act(async () => switchLink("フォーム添削")!.click());
    expect(navigation.replace).not.toHaveBeenCalled();
    expect(navigation.push).not.toHaveBeenCalled();
    expect(replaceState).toHaveBeenLastCalledWith(null, "", pathname);
    expect(panels()[0].hidden).toBe(false);
    expect(panels()[1].hidden).toBe(true);
  });

  it(`${pathname} では行き来しても各一覧の取得は初回の1回だけで、読み込み表示が出ない`, async () => {
    navigation.pathname = pathname;
    await render();
    expect(fetchCount("/v1/coaches")).toBe(1);
    await act(async () => switchLink("継続プラン")!.click());
    await act(async () => { await Promise.resolve(); await Promise.resolve(); });
    expect(fetchCount("/v1/coaches/support-plan")).toBe(1);
    await act(async () => switchLink("フォーム添削")!.click());
    await act(async () => { await Promise.resolve(); await Promise.resolve(); });
    // どちらもマウントし直していないので、再取得も読み込み表示もない。
    expect(fetchCount("/v1/coaches")).toBe(1);
    expect(fetchCount("/v1/coaches/support-plan")).toBe(1);
    expect(container.textContent).not.toContain("読み込み中");
    expect(panels()).toHaveLength(2);
  });
}

it("同じタブのクリックは何もしない", async () => {
  await render();
  replaceState.mockClear();
  await act(async () => switchLink("フォーム添削")!.click());
  expect(replaceState).not.toHaveBeenCalled();
  expect(navigation.replace).not.toHaveBeenCalled();
  expect(navigation.push).not.toHaveBeenCalled();
  expect(fetchCount("/v1/coaches")).toBe(1);
});

it("種目チップで絞り込んだ状態が、タブを行き来しても保持される", async () => {
  await render();
  const target = chip("100m");
  expect(target?.getAttribute("aria-checked")).toBe("false");
  await act(async () => { target!.click(); });
  // 同一ノードの属性が変わる（選び直しではなくその場で切り替わる）。
  expect(target!.getAttribute("aria-checked")).toBe("true");
  await act(async () => switchLink("継続プラン")!.click());
  await act(async () => { await Promise.resolve(); await Promise.resolve(); });
  await act(async () => switchLink("フォーム添削")!.click());
  // フォーム添削の一覧は破棄されていないので、選んだチップのノードも選択も残る。
  expect(panels()[0].hidden).toBe(false);
  const panel = panels()[0];
  expect(
    panel.contains(target!),
    `radios=${container.querySelectorAll('[role="radio"]').length} panels=${panels().length} hidden=${panels().map((p) => p.hidden)} tab=${container.querySelector('[role="tabpanel"]')?.textContent?.slice(0, 160)}`,
  ).toBe(true);
  expect(target!.getAttribute("aria-checked")).toBe("true");
  expect(fetchCount("/v1/coaches")).toBe(1);
});

it("継続プラン側のURLは共有できる（今いるルートのままクエリだけが付く）", async () => {
  navigation.pathname = "/coaching";
  await render();
  expect(switchLink("継続プラン")?.getAttribute("href")).toBe("/coaching?tab=support");
  expect(switchLink("フォーム添削")?.getAttribute("href")).toBe("/coaching");
});

it("継続プラン一覧のフラグが閉じている間は準備中だけを出し、一覧を取得しない", async () => {
  flags.supportList = false;
  navigation.tab = "support";
  await render();
  expect(container.textContent).toContain("継続プランは準備中です");
  expect(container.querySelector('[role="tabpanel"]')?.textContent).toBe("継続プランは準備中です");
  expect(apiFetch.mock.calls.some(([path]) => path === "/v1/coaches/support-plan")).toBe(false);
});

it("狭い画面でも切り替えとコーチボタンは同じ行に収まる", async () => {
  await render();
  const row = coachButton()?.closest("div.flex");
  expect(row).not.toBeNull();
  expect(row?.querySelector('[role="group"]')).not.toBeNull();
});

// 運営者決定（2026-11-14）：ログイン状態・応募有無によらず常時出す。
it("未ログインでもコーチボタンを文言どおり『コーチ』で出し、/coach へ導く（応募状態を取得しない）", async () => {
  auth.token = null;
  await render();
  expect(coachButton()?.getAttribute("href")).toBe("/coach");
  expect(apiFetch.mock.calls.some(([path]) => path === "/v1/coach/me")).toBe(false);
});

it("ログイン済み・応募なしでもコーチボタンを出す（応募状態を取得しない）", async () => {
  await render();
  expect(coachButton()?.getAttribute("href")).toBe("/coach");
  expect(apiFetch.mock.calls.some(([path]) => path === "/v1/coach/me")).toBe(false);
});

it.each(["pending", "approved", "rejected", "suspended"])("応募あり（status=%s）でもコーチボタンを出す（応募状態を取得しない）", async (status) => {
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

it("応募状態の取得を待たず初回描画でコーチボタンを出す", async () => {
  apiFetch.mockReturnValue(new Promise(() => {}));
  await render();
  expect(coachButton()?.getAttribute("href")).toBe("/coach");
});
