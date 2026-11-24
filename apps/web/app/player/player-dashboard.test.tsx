/** @vitest-environment jsdom */
import { act, type ReactNode } from "react";
import { createRoot, type Root } from "react-dom/client";
import { afterEach, beforeEach, expect, it, vi } from "vitest";
import { installEnglish, setActiveLocale } from "@/lib/i18n";
import { en } from "@/lib/i18n-en";

const navigation = vi.hoisted(() => ({ replace: vi.fn() }));
const auth = vi.hoisted(() => ({ session: {} as object | null, loading: false, token: "tok" as string | null }));
const apiFetch = vi.hoisted(() => vi.fn());
vi.mock("next/navigation", () => ({ useRouter: () => ({ replace: navigation.replace }) }));
vi.mock("@/components/auth-provider", () => ({ useAuth: () => ({ session: auth.session, loading: auth.loading, token: auth.token }) }));
vi.mock("@/lib/api", async (original) => ({ ...await original<typeof import("@/lib/api")>(), apiFetch: (...args: unknown[]) => apiFetch(...args) }));
vi.mock("next/link", () => ({
  default: ({ href, children, ...props }: { href: string; children: ReactNode }) => (
    <a href={href} {...props}>{children}</a>
  ),
}));
const { PlayerDashboard } = await import("./player-dashboard");

let root: Root;
let container: HTMLDivElement;
beforeEach(() => {
  (globalThis as typeof globalThis & { IS_REACT_ACT_ENVIRONMENT: boolean }).IS_REACT_ACT_ENVIRONMENT = true;
  container = document.createElement("div");
  document.body.appendChild(container);
  root = createRoot(container);
  navigation.replace.mockReset();
  auth.session = {};
  auth.loading = false;
  auth.token = "tok";
  apiFetch.mockReset();
  // 既定は全タイル 0 件（円ボタンの player_count などの他キーも返る想定）。
  apiFetch.mockResolvedValue(counts({}));
  setActiveLocale("ja");
});
afterEach(async () => {
  await act(async () => root.unmount());
  container.remove();
  setActiveLocale("ja");
});
const render = async () => {
  await act(async () => root.render(<PlayerDashboard />));
  await act(async () => { await Promise.resolve(); });
};
const tiles = () => [...container.querySelectorAll("a")];
const counts = (c: Partial<Record<"one" | "personal" | "support" | "ticket", number>>) => ({
  player_count: 9,
  coach_count: 4,
  player_one_time_count: c.one ?? 0,
  player_personal_count: c.personal ?? 0,
  player_support_count: c.support ?? 0,
  player_ticket_count: c.ticket ?? 0,
});
const badges = () => tiles().map((a) => a.querySelector("span[aria-hidden]")?.textContent ?? null);
const labels = () => tiles().map((a) => a.querySelector(".coach-tile-label")?.textContent);
const hrefs = ["/me/coaching/requests", "/me/coaching/tickets", "/me/push"];
const badgeClass = "absolute -right-1 -top-1 flex h-5 min-w-5 items-center justify-center rounded-full bg-red-500 px-1 text-[11px] font-bold text-white";

it("ログイン済みでは見出し『選手』と3つのタイル（スタンダードプラン・チケット・通知）が順番どおりに出る（本体 #1007 B4）", async () => {
  await render();
  expect(container.querySelector("h1")?.textContent).toBe("選手");
  expect(tiles().map((a) => a.textContent)).toEqual(["スタンダードプラン", "チケット", "通知"]);
  expect(tiles().map((a) => a.getAttribute("href"))).toEqual(hrefs);
  // 継続プランは休眠。継続プラン・無料相談のタイルは出さない。
  expect(container.querySelector('a[href="/me/coaching/personal"]')).toBeNull();
  expect(container.querySelector('a[href="/coaching/support-consults"]')).toBeNull();
  expect(container.textContent).not.toContain("継続プラン");
  expect(container.textContent).not.toContain("無料相談");
  expect(navigation.replace).not.toHaveBeenCalled();
});

it("タイルはコーチ画面と同じ .coach-tiles > nav の構造で、3列の正方形で並べる（#940）", async () => {
  await render();
  const wrapper = container.querySelector(".coach-tiles");
  expect(wrapper).not.toBeNull();
  expect(wrapper!.className).toBe("coach-tiles");
  const nav = wrapper!.firstElementChild!;
  expect(nav.tagName).toBe("NAV");
  expect(wrapper!.children).toHaveLength(1);
  expect(nav.className).toBe("grid gap-2");
  expect(nav.className).not.toContain("grid-cols-3");
  expect(nav.getAttribute("aria-label")).toBe("選手");
  const links = [...nav.children];
  expect(links).toHaveLength(3);
  for (const tile of links) {
    expect(tile.tagName).toBe("A");
    expect(tile.className).toContain("aspect-square");
  }
  // 縦配分（CoachTiles の ResizeObserver）は持ち込まないので、インラインの変数指定も無い。
  expect(nav.getAttribute("style")).toBeNull();
  expect(wrapper!.getAttribute("style")).toBeNull();
});

it("タイルのラベルはコーチ側と同じ coach-tile-label を持ち、折り返さず、タイルに固定の大きさは無い（#940）", async () => {
  await render();
  const labels = [...container.querySelectorAll(".coach-tiles > nav > a > span.coach-tile-label")];
  expect(labels).toHaveLength(3);
  for (const label of labels) {
    expect(label.className).toBe("coach-tile-label whitespace-nowrap text-[11px] font-semibold leading-tight");
  }
  for (const tile of tiles()) {
    expect(tile.className.split(/\s+/).filter((c) => /^(w|h|min-w|min-h|max-w|max-h)-/.test(c))).toEqual([]);
  }
});

// #1003：#940 の「タイルにバッジ・ドットは出ない」を置き換え。件数の無いときは出ないことを、モックした件数で確かめる。
it("件数があるタイルにだけ、その数字の赤バッジが出る（通知タイルには出ない。継続プラン・無料相談の件数はタイルが無いので出ない）", async () => {
  apiFetch.mockResolvedValue(counts({ one: 2, personal: 1, support: 0, ticket: 3 }));
  await render();
  expect(apiFetch).toHaveBeenCalledWith("/v1/me/coaching/action-counts", { token: "tok" });
  expect(badges()).toEqual(["2", "3", null]);
  for (const tile of tiles()) {
    const badge = tile.querySelector("span[aria-hidden]");
    if (badge) expect(badge.className).toBe(badgeClass);
    // ドット（オレンジ点）は選手側には付けない。
    expect(tile.querySelectorAll("span").length).toBeLessThanOrEqual(2);
  }
  // 順序・行き先・ラベルは従来どおり。
  expect(labels()).toEqual(["スタンダードプラン", "チケット", "通知"]);
  expect(tiles().map((a) => a.getAttribute("href"))).toEqual(hrefs);
});

it("件数は丸めずにそのまま出す（99+ にしない）", async () => {
  apiFetch.mockResolvedValue(counts({ one: 120 }));
  await render();
  expect(badges()[0]).toBe("120");
});

it("件数が全て 0 のときはバッジを出さず、タイルはラベルのみ", async () => {
  await render();
  expect(apiFetch).toHaveBeenCalledTimes(1);
  expect(badges()).toEqual([null, null, null]);
  for (const tile of tiles()) {
    expect(tile.querySelectorAll("span")).toHaveLength(1);
    expect(tile.getAttribute("aria-label")).toBeNull();
  }
  expect(container.textContent).not.toMatch(/\d+件/);
});

it("取得に失敗してもバッジを出さないだけで、タイルは従来どおり表示・遷移できる", async () => {
  apiFetch.mockRejectedValue(new Error("boom"));
  await render();
  expect(badges()).toEqual([null, null, null]);
  expect(tiles().map((a) => a.getAttribute("href"))).toEqual(hrefs);
  expect(labels()).toEqual(["スタンダードプラン", "チケット", "通知"]);
  expect(navigation.replace).not.toHaveBeenCalled();
});

it("タイルは件数の取得を待たずに出し、届いたらバッジが付く", async () => {
  let resolve: (v: unknown) => void = () => {};
  apiFetch.mockReturnValue(new Promise((r) => { resolve = r; }));
  await render();
  expect(tiles()).toHaveLength(3);
  expect(container.querySelector(".page-loading-screen")).toBeNull();
  expect(badges()).toEqual([null, null, null]);
  await act(async () => { resolve(counts({ ticket: 4 })); await Promise.resolve(); });
  expect(badges()).toEqual([null, "4", null]);
});

it("画面の再表示（visibilitychange／focus）で取り直す", async () => {
  await render();
  expect(apiFetch).toHaveBeenCalledTimes(1);
  apiFetch.mockResolvedValue(counts({ ticket: 1 }));
  await act(async () => { document.dispatchEvent(new Event("visibilitychange")); await Promise.resolve(); });
  expect(apiFetch).toHaveBeenCalledTimes(2);
  expect(badges()).toEqual([null, "1", null]);
  await act(async () => { window.dispatchEvent(new Event("focus")); await Promise.resolve(); });
  expect(apiFetch).toHaveBeenCalledTimes(3);
});

it("token が無いときは取得せず、バッジも出さない", async () => {
  auth.token = null;
  apiFetch.mockResolvedValue(counts({ one: 5 }));
  await render();
  expect(apiFetch).not.toHaveBeenCalled();
  expect(badges()).toEqual([null, null, null]);
});

it("件数があるタイルの aria-label は「{ラベル}（{件数}件）」、0件タイルはラベルのみ", async () => {
  apiFetch.mockResolvedValue(counts({ one: 2, ticket: 3 }));
  await render();
  expect(tiles().map((a) => a.getAttribute("aria-label"))).toEqual(["スタンダードプラン（2件）", "チケット（3件）", null]);
});

it("英語では aria-label が \"{label} ({count})\" になる", async () => {
  installEnglish(en);
  setActiveLocale("en");
  apiFetch.mockResolvedValue(counts({ one: 2, ticket: 3 }));
  await render();
  expect(tiles().map((a) => a.getAttribute("aria-label"))).toEqual(["Standard Plan (2)", "Tickets (3)", null]);
});

it("未ログインでは /login?next=/player へ寄せ、タイルを出さない", async () => {
  auth.session = null;
  await render();
  expect(navigation.replace).toHaveBeenCalledWith("/login?next=/player");
  expect(tiles()).toHaveLength(0);
  expect(container.querySelector(".page-loading-screen")).not.toBeNull();
});

it("読み込み中は PageLoading だけを出し、リダイレクトしない", async () => {
  auth.session = null;
  auth.loading = true;
  await render();
  expect(container.querySelector(".page-loading-screen")).not.toBeNull();
  expect(tiles()).toHaveLength(0);
  expect(navigation.replace).not.toHaveBeenCalled();
});

it("英語ではタイル名が既存キーの訳になる", async () => {
  installEnglish(en);
  setActiveLocale("en");
  await render();
  expect(container.querySelector("h1")?.textContent).toBe("Athlete");
  expect(tiles().map((a) => a.textContent)).toEqual(["Standard Plan", "Tickets", "Notifications"]);
});
