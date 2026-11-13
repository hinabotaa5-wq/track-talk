/** @vitest-environment jsdom */
import { act, type ReactNode } from "react";
import { createRoot, type Root } from "react-dom/client";
import { afterEach, beforeEach, expect, it, vi } from "vitest";
import { CoachDashboard } from "./coach-dashboard";
import { installEnglish, setActiveLocale } from "@/lib/i18n";
import { en } from "@/lib/i18n-en";

const apiFetch = vi.fn();
const flags = vi.hoisted(() => ({ enabled: true, locale: undefined as string | undefined }));
vi.mock("@/lib/api", async (original) => ({
  ...await original<typeof import("@/lib/api")>(),
  apiFetch: (...args: unknown[]) => apiFetch(...args),
}));
vi.mock("@/lib/features", async (original) => ({
  ...await original<typeof import("@/lib/features")>(),
  get coachingEnabled() {
    return flags.enabled;
  },
}));
vi.mock("@/components/auth-provider", () => ({
  useAuth: () => ({ session: {}, profile: { nickname: "テスト", locale: flags.locale }, token: "test", loading: false }),
}));
vi.mock("next/navigation", () => ({ useRouter: () => ({ replace: vi.fn() }) }));
// 表示言語は LocaleProvider が決める。ここでは利用者の言語（flags.locale）をそのまま使う。
vi.mock("@/components/locale-provider", () => ({ useLocale: () => (flags.locale === "en" ? "en" : "ja") }));
vi.mock("next/link", () => ({
  default: ({ href, children, ...props }: { href: string; children: ReactNode }) => (
    <a href={href} {...props}>{children}</a>
  ),
}));

const application = {
  name: "テストコーチ",
  service_types: ["video_review"],
  tier: "coach",
  events: ["100m"],
  status: "approved",
  applied_at: "2026-01-01T00:00:00Z",
  reviewed_at: null,
  on_duty: true,
  daily_cap: 5,
  photo_url: "",
  pledged: true,
  payouts_ready: false,
};

function reviewRequest(id: string) {
  return {
    id,
    event: "100m",
    tier: "coach",
    question: "フォームを見てください",
    focus: [],
    status: "in_review",
    due_at: "2026-12-01T00:00:00Z",
    requested_coach_id: "coach-1",
    coach_reward_yen: 1050,
  };
}

function mockAll(overrides: Record<string, unknown> = {}) {
  apiFetch.mockImplementation((path: string) => {
    if (path === "/v1/coach/me") {
      return Promise.resolve({ application, rating: { count: 3, average: 4.5, paused: false } });
    }
    if (path === "/v1/coach/requests") {
      return Promise.resolve({
        offers: [
          { id: "o1", expires_at: new Date(Date.now() + 50 * 60000).toISOString(), request: reviewRequest("r0") },
          { id: "o2", expires_at: new Date(Date.now() + 80 * 60000).toISOString(), request: reviewRequest("r0b") },
        ],
        reviewing: [reviewRequest("r1")],
      });
    }
    if (path === "/v1/coach/consultations") return Promise.resolve({ consultations: [{ id: "c1" }] });
    if (path === "/v1/coach/contracts") {
      return Promise.resolve({ contracts: [{ id: "k1", status: "applied" }, { id: "k2", status: "active" }] });
    }
    if (path === "/v1/coach/me/earnings") {
      return Promise.resolve({ unpaid_yen: 1050, next_payout_date: "2026-12-15", next_payout_yen: 1050 });
    }
    if (path === "/v1/coach/me/identity-status") return Promise.resolve({ registered: false, applies: true });
    if (path in overrides) return Promise.resolve(overrides[path]);
    return Promise.reject(new Error(`unexpected ${path}`));
  });
}

let root: Root;
let container: HTMLDivElement;
beforeEach(() => {
  (globalThis as typeof globalThis & { IS_REACT_ACT_ENVIRONMENT: boolean }).IS_REACT_ACT_ENVIRONMENT = true;
  apiFetch.mockReset();
  flags.enabled = true;
  flags.locale = undefined;
  setActiveLocale("ja");
  container = document.createElement("div");
  document.body.appendChild(container);
  root = createRoot(container);
});
afterEach(async () => {
  await act(async () => root.unmount());
  container.remove();
});

function tileLinks() {
  return [...container.querySelectorAll("nav.grid a")];
}

function tileHrefs() {
  return tileLinks().map((a) => a.getAttribute("href"));
}

function tileLabels() {
  return tileLinks().map((a) => a.getAttribute("aria-label"));
}

// 取得はマウント時に走るので、条件を変える時は掛け直す。
async function renderFresh() {
  await act(async () => root.unmount());
  container.remove();
  container = document.createElement("div");
  document.body.appendChild(container);
  root = createRoot(container);
  await act(async () => root.render(<CoachDashboard />));
}

it("ランチャーは9タイルの入口だけで（カレンダーは廃止）、中身のセクションを出さない", async () => {
  mockAll();
  await act(async () => root.render(<CoachDashboard />));
  expect(tileHrefs()).toEqual([
    "/coach/reviewing",
    "/coach/consultations",
    "/coach/contracts",
    "/coach/support-consults",
    "/coach/plans",
    "/coach/commerce",
    "/coach/payouts",
    "/coach/profile",
    "/me/push",
  ]);
  // 中身は個別画面に切り出したので、ランチャーには出さない。
  expect(container.textContent).not.toContain("引き受ける");
  expect(container.textContent).not.toContain("見送る");
  expect(container.textContent).not.toContain("対応待ちの追加相談");
  expect(container.textContent).not.toContain("受け取り口座を登録する");
  expect(container.querySelector('a[href="/me/coaching"]')).toBeNull();
  // ステータスバーは従来どおり操作できる。
  expect(container.querySelector('[role="switch"][aria-label="受付中"]')?.getAttribute("aria-checked")).toBe("true");
  expect(container.querySelector("select")).toBeNull();
  expect(container.textContent).toContain("★4.5（3件）");
  // フッターは要約1行とやめる導線だけ。
  expect(container.textContent).toContain("未払い1,050円");
  expect(container.querySelector('a[href="/coach/withdraw"]')).not.toBeNull();
});

it("バッジは実件数と一致し、未登録の点は条件の時だけ出す", async () => {
  mockAll();
  await act(async () => root.render(<CoachDashboard />));
  expect(tileLabels()).toContain("対応中（1件）");
  expect(tileLabels()).toContain("追加相談（1件）");
  expect(tileLabels()).toContain("契約・申込み（1件）");
  expect(tileLabels()).toContain("特商法・身元（要確認）");
  expect(tileLabels()).toContain("報酬・口座（要確認）");

  // 身元が不明（null）や取得失敗のときは点を出さない。
  for (const status of [{ registered: null, applies: true }, { registered: false, applies: false }]) {
    apiFetch.mockImplementation((path: string) => {
      if (path === "/v1/coach/me") return Promise.resolve({ application: { ...application, payouts_ready: true } });
      if (path === "/v1/coach/requests") return Promise.resolve({ offers: [], reviewing: [] });
      if (path === "/v1/coach/consultations") return Promise.resolve({ consultations: [] });
      if (path === "/v1/coach/contracts") return Promise.resolve({ contracts: [] });
      if (path === "/v1/coach/me/earnings") return Promise.resolve({ unpaid_yen: 0 });
      if (path === "/v1/coach/me/identity-status") return Promise.resolve(status);
      return Promise.reject(new Error(`unexpected ${path}`));
    });
    await renderFresh();
    expect(tileLabels()).toContain("特商法・身元");
    expect(tileLabels()).not.toContain("特商法・身元（要確認）");
    expect(tileLabels()).not.toContain("報酬・口座（要確認）");
  }

  // 取得失敗でも入口は壊れない。
  apiFetch.mockImplementation((path: string) => {
    if (path === "/v1/coach/me") return Promise.resolve({ application });
    return Promise.reject(new Error("down"));
  });
  await renderFresh();
  expect(tileHrefs()).toHaveLength(9);
});

it("未承認の表示は現行のまま変えない", async () => {
  apiFetch.mockResolvedValue({ application: { ...application, status: "pending" } });
  await act(async () => root.render(<CoachDashboard />));
  expect(container.textContent).toContain("コーチングは準備中です");
  expect(container.querySelector('nav[aria-label="コーチのメニュー"]')).toBeNull();
});

it("言語が英語の利用者にはコーチ導線を英語で出す", async () => {
  flags.locale = "en";
  installEnglish(en);
  setActiveLocale("en");
  mockAll();
  await act(async () => root.render(<CoachDashboard />));
  expect(container.querySelector('nav[aria-label="Coach menu"]')).not.toBeNull();
  expect(tileLabels()).toContain("Reviewing (1)");
  // 日本語の文言は出ない。
  expect(container.textContent).not.toContain("届いた依頼");
  expect(container.textContent).not.toContain("新しい依頼が");
});

it("ランチャーは高さいっぱいに広がる枠で描き、タイルは3列の正方形で並べる（#580）", async () => {
  mockAll();
  await act(async () => root.render(<CoachDashboard />));
  const launcher = container.querySelector(".coach-launcher");
  expect(launcher).not.toBeNull();
  expect(launcher!.className).toContain("max-md:flex-1");
  const grid = container.querySelector("nav.grid")!;
  expect(grid.parentElement?.className).toBe("coach-tiles");
  expect(grid.parentElement?.parentElement).toBe(launcher);
  for (const tile of tileLinks()) expect(tile.className).toContain("aspect-square");
  expect(launcher!.lastElementChild?.tagName).toBe("FOOTER");
});

it("タイルは横幅駆動で固定の大きさを持たず、ラベルは折り返さない（#580）", async () => {
  mockAll();
  await act(async () => root.render(<CoachDashboard />));
  const grid = container.querySelector("nav.grid")!;
  expect(grid.getAttribute("style") ?? "").not.toContain("--coach-tile");
  const labels = [...container.querySelectorAll("nav.grid a > span.coach-tile-label")];
  expect(labels).toHaveLength(9);
  for (const label of labels) expect(label.className).toContain("whitespace-nowrap");
});
