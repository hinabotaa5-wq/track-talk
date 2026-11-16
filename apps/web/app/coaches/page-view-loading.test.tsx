/** @vitest-environment jsdom */
import { act, type ReactNode } from "react";
import { createRoot, type Root } from "react-dom/client";
import { afterEach, beforeEach, expect, it, vi } from "vitest";
import { installEnglish, setActiveLocale } from "@/lib/i18n";
import { en } from "@/lib/i18n-en";

// 本体 #804：コーチ一覧の初回読み込み中は、ページ中身をストップウォッチだけにする。
// 紹介バナー・タブ切り替え・「コーチ」ボタン・種目チップは、表示中タブの一覧の初回取得が終わるまで DOM に出さない。
const navigation = vi.hoisted(() => ({ tab: null as string | null, pathname: "/coaches" }));
const auth = { token: "tok" as string | null };
const flags = { supportList: true };
const apiFetch = vi.fn();
vi.mock("next/navigation", () => ({
  useRouter: () => ({ replace: vi.fn() }),
  useSearchParams: () => ({ get: (key: string) => (key === "tab" ? navigation.tab : null) }),
  usePathname: () => navigation.pathname,
}));
vi.mock("next/link", () => ({ default: ({ href, children }: { href: string; children: ReactNode }) => <a href={href}>{children}</a> }));
vi.mock("@/lib/api", async (original) => ({ ...await original<typeof import("@/lib/api")>(), apiFetch: (...args: unknown[]) => apiFetch(...args) }));
vi.mock("@/lib/features", async (original) => ({ ...await original<typeof import("@/lib/features")>(), get supportPlanListEnabled() { return flags.supportList; } }));
vi.mock("@/components/auth-provider", () => ({ useAuth: () => ({ token: auth.token, loading: false }) }));
const { CoachesView } = await import("./page-view");

let root: Root;
let container: HTMLDivElement;
let finishCoaches!: (value: { coaches: never[] }) => void;
let finishSupport!: (value: { coaches: never[] }) => void;
beforeEach(() => {
  (globalThis as typeof globalThis & { IS_REACT_ACT_ENVIRONMENT: boolean }).IS_REACT_ACT_ENVIRONMENT = true;
  container = document.createElement("div"); document.body.appendChild(container); root = createRoot(container);
  apiFetch.mockReset(); navigation.tab = null; navigation.pathname = "/coaches";
  auth.token = "tok"; flags.supportList = true;
  apiFetch.mockImplementation((path: string) => {
    if (path === "/v1/coaches") return new Promise<{ coaches: never[] }>((resolve) => { finishCoaches = resolve; });
    if (path === "/v1/coaches/support-plan") return new Promise<{ coaches: never[] }>((resolve) => { finishSupport = resolve; });
    // 円ボタンのバッジ用の件数（本体 #891）。一覧の読み込みとは別で、ここでは 0 件。
    if (path === "/v1/me/coaching/action-counts") return Promise.resolve({ player_count: 0, coach_count: 0 });
    throw new Error(`unexpected ${path}`);
  });
});
afterEach(async () => { await act(async () => root.unmount()); container.remove(); setActiveLocale("ja"); });
const render = async () => {
  await act(async () => root.render(<CoachesView />));
  await act(async () => { await Promise.resolve(); });
};
const stopwatches = () => [...container.querySelectorAll('[role="status"]')];

// §5.1：初回読み込み中（1回プラン）はストップウォッチだけになる。
it("初回読み込み中（1回プラン）はストップウォッチだけで、バナー・タブ・ボタン・チップは出ない", async () => {
  await render();
  expect(stopwatches()).toHaveLength(1);
  expect(container.querySelector(".page-loading-screen")).not.toBeNull();
  expect(container.textContent).not.toContain("友達2人に紹介すると、1回無料！");
  expect(container.textContent).not.toContain("1回プランの都度払いやチケットの購入で、紹介IDが発行されます。");
  expect(container.textContent).not.toContain("1回プラン");
  expect(container.textContent).not.toContain("継続プラン");
  expect(container.textContent).not.toContain("コーチ");
  expect(container.querySelector('[role="radio"]')).toBeNull();
});

// §5.2：初回読み込み中（継続プラン）はストップウォッチだけになる。
it("初回読み込み中（継続プラン）はストップウォッチだけで、バナー・タブ・ボタン・チップは出ない", async () => {
  navigation.tab = "support";
  await render();
  expect(stopwatches()).toHaveLength(1);
  expect(container.querySelector(".page-loading-screen")).not.toBeNull();
  expect(container.textContent).not.toContain("友達2人に紹介すると、1回無料！");
  expect(container.textContent).not.toContain("1回プラン");
  expect(container.textContent).not.toContain("継続プラン");
  expect(container.querySelector('[role="radio"]')).toBeNull();
  expect(apiFetch.mock.calls.some(([path]) => path === "/v1/coaches/support-plan")).toBe(true);
  // 保留した応答を片付ける（このテストでは読み込み中の表示だけを検証する）。
  await act(async () => finishSupport({ coaches: [] }));
});

// §5.1（英語）：英語でも初回読み込み中はストップウォッチだけになる。
it("英語でも初回読み込み中はストップウォッチだけで、バナー・タブは出ない", async () => {
  installEnglish(en);
  setActiveLocale("en");
  await render();
  expect(stopwatches()).toHaveLength(1);
  expect(container.textContent).not.toContain("Refer two friends and get a free review!");
  expect(container.textContent).not.toContain("One-time Plan");
  expect(container.textContent).not.toContain("Ongoing plan");
});

// §5.3：読み込み完了後はバナー＋タブ切り替え＋一覧が従来どおり表示され、ストップウォッチが消える。
it("読み込み完了後はバナー・タブ切り替え・一覧が出て、ストップウォッチが消える", async () => {
  await render();
  await act(async () => finishCoaches({ coaches: [] }));
  await act(async () => { await Promise.resolve(); await Promise.resolve(); });
  expect(container.querySelector(".page-loading-screen")).toBeNull();
  expect(container.textContent).toContain("友達2人に紹介すると、1回無料！");
  expect(container.textContent).not.toContain("購入すると、紹介IDが発行されます。そのIDで友達2人が購入すると");
  expect(container.textContent).toContain("1回プラン");
  expect(container.textContent).toContain("継続プラン");
  expect(container.querySelector('[role="radio"]')).not.toBeNull();
  expect(apiFetch.mock.calls.filter(([path]) => path === "/v1/coaches")).toHaveLength(1);
});

// §5.4：エラー時はストップウォッチが消え、従来のエラー表示に到達する。
it("取得に失敗したらストップウォッチが消え、エラー表示になる", async () => {
  apiFetch.mockRejectedValueOnce(new Error("network down"));
  await render();
  expect(container.querySelector(".page-loading-screen")).toBeNull();
  expect(container.querySelector('[role="alert"]')?.textContent).toBe("読み込みに失敗しました");
});
