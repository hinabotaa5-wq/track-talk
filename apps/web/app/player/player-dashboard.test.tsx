/** @vitest-environment jsdom */
import { act, type ReactNode } from "react";
import { createRoot, type Root } from "react-dom/client";
import { afterEach, beforeEach, expect, it, vi } from "vitest";
import { installEnglish, setActiveLocale } from "@/lib/i18n";
import { en } from "@/lib/i18n-en";

const navigation = vi.hoisted(() => ({ replace: vi.fn() }));
const auth = vi.hoisted(() => ({ session: {} as object | null, loading: false }));
vi.mock("next/navigation", () => ({ useRouter: () => ({ replace: navigation.replace }) }));
vi.mock("@/components/auth-provider", () => ({ useAuth: () => ({ session: auth.session, loading: auth.loading }) }));
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

it("ログイン済みでは見出し『選手』と5つのタイルが順番どおりに出る", async () => {
  await render();
  expect(container.querySelector("h1")?.textContent).toBe("選手");
  expect(tiles().map((a) => a.textContent)).toEqual(["1回プラン", "継続プラン", "無料相談", "チケット", "通知"]);
  expect(tiles().map((a) => a.getAttribute("href"))).toEqual([
    "/me/coaching/requests",
    "/me/coaching/personal",
    "/coaching/support-consults",
    "/me/coaching/tickets",
    "/me/push",
  ]);
  expect(navigation.replace).not.toHaveBeenCalled();
});

it("タイルにバッジ・ドットは出ない", async () => {
  await render();
  for (const tile of tiles()) {
    // アイコンとラベルだけ（バッジ・ドットの span を足さない）。
    expect(tile.querySelectorAll("span")).toHaveLength(1);
  }
  expect(container.textContent).not.toMatch(/\d+件/);
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
  expect(tiles().map((a) => a.textContent)).toEqual(["One-time Plan", "Ongoing plan", "Free consults", "Tickets", "Notifications"]);
});
