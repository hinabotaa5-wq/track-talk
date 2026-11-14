/** @vitest-environment jsdom */
import { act, createElement, type ReactNode } from "react";
import { createRoot, type Root } from "react-dom/client";
import { afterEach, beforeEach, expect, it, vi } from "vitest";
import CoachingPage, { metadata } from "./page";

// /coaching はカードメニューではなく、/coaches と同じ一覧画面を出す（本体 #641）。
// 旧ハブの文言（カード・利用中の方・準備中の説明）は一覧側に出さない。
const apiFetch = vi.fn();
const flags = { coaching: false, list: false, supportList: true };
const navigation = { tab: null as string | null };
vi.mock("@/lib/features", async (original) => ({
  ...await original<typeof import("@/lib/features")>(),
  get coachingEnabled() { return flags.coaching; },
  get coachingCoachListEnabled() { return flags.list; },
  get supportPlanListEnabled() { return flags.supportList; },
}));
vi.mock("next/navigation", () => ({
  useRouter: () => ({ replace: vi.fn() }),
  useSearchParams: () => ({ get: (key: string) => (key === "tab" ? navigation.tab : null) }),
  usePathname: () => "/coaching",
}));
vi.mock("next/link", () => ({
  default: ({ children, href, ...props }: { children: ReactNode; href: string }) => createElement("a", { href, ...props }, children),
}));
vi.mock("@/lib/api", async (original) => ({ ...await original<typeof import("@/lib/api")>(), apiFetch: (...args: unknown[]) => apiFetch(...args) }));
vi.mock("@/components/auth-provider", () => ({ useAuth: () => ({ token: null, loading: false }) }));

let root: Root;
let container: HTMLDivElement;
beforeEach(() => {
  (globalThis as typeof globalThis & { IS_REACT_ACT_ENVIRONMENT: boolean }).IS_REACT_ACT_ENVIRONMENT = true;
  container = document.createElement("div"); document.body.appendChild(container); root = createRoot(container);
  flags.coaching = false; flags.list = false; flags.supportList = true;
  navigation.tab = null;
  apiFetch.mockReset();
  apiFetch.mockImplementation(async (path: string) => {
    if (path === "/v1/coaches") return { coaches: [] };
    if (path === "/v1/coaches/support-plan") return { coaches: [] };
    throw new Error(`unexpected ${path}`);
  });
});
afterEach(async () => { await act(async () => root.unmount()); container.remove(); vi.unstubAllEnvs(); });
const render = async () => { await act(async () => root.render(<CoachingPage />)); await act(async () => { await Promise.resolve(); }); };
const hubWords = ["利用中の方", "コーチを選んで動画を送る", "登録しているコーチを見る", "コーチングは準備中です", "コーチ募集・応募状況の確認", "準備中・公開までお待ちください"];

it("メタ説明は旧ハブ前提（準備中・コーチ募集）をやめ、一覧の実態に合わせる", () => {
  expect(metadata.description).not.toContain("準備中");
  expect(metadata.description).not.toContain("募集");
  expect(metadata.description).toContain("コーチの一覧");
});

it("コーチングと一覧の両方を開けると、カードメニューではなく一覧を出す", async () => {
  flags.coaching = true;
  flags.list = true;
  await render();
  // 実リスト（空なので空状態）が出て、旧ハブの文言は出ない。
  expect(container.textContent).toContain("まだいません");
  expect(container.querySelector('[role="tabpanel"]')).not.toBeNull();
  expect(container.querySelector("h1")).toBeNull();
  for (const word of hubWords) expect(container.textContent).not.toContain(word);
});

it("?tab=support で直接開くと継続プラン側を出す", async () => {
  flags.coaching = true;
  flags.list = true;
  navigation.tab = "support";
  await render();
  expect(container.querySelector('[role="tabpanel"]')).not.toBeNull();
  for (const word of hubWords) expect(container.textContent).not.toContain(word);
});

it.each([[false, true], [true, false], [false, false]])("一覧フラグとコーチングのどちらかが閉じている間は準備中だけを出し、一覧を取得しない（coaching=%s, list=%s）", async (coaching, list) => {
  flags.coaching = coaching;
  flags.list = list;
  await render();
  expect(container.textContent).toContain("ただいま準備中です");
  expect(container.querySelector('[role="tabpanel"]')).toBeNull();
  for (const word of hubWords) expect(container.textContent).not.toContain(word);
  expect(apiFetch.mock.calls.some(([path]) => path === "/v1/coaches")).toBe(false);
  expect(apiFetch.mock.calls.some(([path]) => path === "/v1/coaches/support-plan")).toBe(false);
});

it("開始の告知がある間も、閉じているときだけ告知と準備中を出す", async () => {
  vi.stubEnv("NEXT_PUBLIC_VIDEO_REVIEW_NOTICE", "11/14(土)に開始します！");
  await render();
  expect(container.textContent).toContain("11/14(土)に開始します！");
  expect(container.textContent).toContain("ただいま準備中です");
});
