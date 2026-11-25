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

it("ランチャーは8タイルの入口だけで（カレンダーは廃止）、中身のセクションを出さない", async () => {
  mockAll();
  await act(async () => root.render(<CoachDashboard />));
  expect(tileHrefs()).toEqual([
    "/coach/reviewing",
    "/coach/consultations",
    "/coach/contracts",
    "/coach/payouts",
    "/coach/profile",
    "/me/push",
    "/coach/feedback",
    "/coach/history",
  ]);
  // 中身は個別画面に切り出したので、ランチャーには出さない。
  expect(container.textContent).not.toContain("引き受ける");
  expect(container.textContent).not.toContain("見送る");
  expect(container.textContent).not.toContain("対応待ちの追加相談");
  expect(container.textContent).not.toContain("受け取り口座を登録する");
  expect(container.querySelector('a[href="/me/coaching"]')).toBeNull();
  // 本体 #1007 B5・B11：継続プランは休眠。届いた相談・継続プラン・特商法・身元のタイルは出さず、身元登録の状態も取りに行かない。
  for (const href of ["/coach/plans", "/coach/support-consults", "/coach/commerce"]) expect(container.querySelector(`a[href="${href}"]`)).toBeNull();
  expect(apiFetch.mock.calls.some(([path]) => path === "/v1/coach/me/identity-status")).toBe(false);
  // ステータスバーは従来どおり操作できる。
  expect(container.querySelector('[role="switch"][aria-label="受付中"]')?.getAttribute("aria-checked")).toBe("true");
  expect(container.querySelector("select")).toBeNull();
  // 星評価は廃止：API が評価を返しても平均・件数は出さない。
  expect(container.textContent).not.toContain("★4.5（3件）");
  expect(container.textContent).not.toContain("★");
  // フッターは要約1行とやめる導線だけ。
  expect(container.textContent).toContain("未払い1,050円");
  expect(container.querySelector('a[href="/coach/withdraw"]')).not.toBeNull();
});

it("paused が true でも停止注記は出ない（評価制度は廃止）", async () => {
  mockAll({ "/v1/coach/me": { application, rating: { count: 8, average: 2.1, paused: true } } });
  await act(async () => root.render(<CoachDashboard />));
  expect(container.textContent).not.toContain("依頼が回ってきません");
  expect(container.querySelector('[role="switch"][aria-label="受付中"]')?.getAttribute("aria-checked")).toBe("true");
});

it("バッジは実件数と一致し、口座未登録の点は条件の時だけ出す", async () => {
  mockAll();
  await act(async () => root.render(<CoachDashboard />));
  expect(tileLabels()).toContain("対応中（1件）");
  expect(tileLabels()).toContain("追加相談（1件）");
  expect(tileLabels()).toContain("契約・申込み（1件）");
  expect(tileLabels()).toContain("報酬・口座（要確認）");
  // 特商法・身元のタイルは出さない（本体 #1007）。
  expect(tileLabels().some((label) => label?.startsWith("特商法・身元"))).toBe(false);

  // 口座が登録済みなら点を出さない。
  apiFetch.mockImplementation((path: string) => {
    if (path === "/v1/coach/me") return Promise.resolve({ application: { ...application, payouts_ready: true } });
    if (path === "/v1/coach/requests") return Promise.resolve({ offers: [], reviewing: [] });
    if (path === "/v1/coach/consultations") return Promise.resolve({ consultations: [] });
    if (path === "/v1/coach/contracts") return Promise.resolve({ contracts: [] });
    if (path === "/v1/coach/me/earnings") return Promise.resolve({ unpaid_yen: 0 });
    return Promise.reject(new Error(`unexpected ${path}`));
  });
  await renderFresh();
  expect(tileLabels()).not.toContain("報酬・口座（要確認）");

  // 取得失敗でも入口は壊れない。
  apiFetch.mockImplementation((path: string) => {
    if (path === "/v1/coach/me") return Promise.resolve({ application });
    return Promise.reject(new Error("down"));
  });
  await renderFresh();
  expect(tileHrefs()).toHaveLength(8);
});

it("未承認時はコーチ募集の文言を表示する", async () => {
  apiFetch.mockResolvedValue({ application: { ...application, status: "pending" } });
  await act(async () => root.render(<CoachDashboard />));
  expect(container.textContent).toContain("競技や指導の経験を生かして、選手の次の一歩を支えるコーチを募集しています。");
  expect(container.textContent).not.toContain("コーチングは準備中です");
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
  expect(tileLabels()).toContain("Feedback from athletes");
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
  expect(labels).toHaveLength(8);
  for (const label of labels) expect(label.className).toContain("whitespace-nowrap");
});

// 選手からの匿名の感想（本体 #911）。既存タイルの後ろに1つ足し、件数バッジは付けない。
it("「選手からの感想」タイルがあり、/coach/feedback へ遷移する", async () => {
  mockAll();
  await act(async () => root.render(<CoachDashboard />));
  const tile = tileLinks().find((a) => a.getAttribute("href") === "/coach/feedback");
  expect(tile).toBeDefined();
  expect(tile!.getAttribute("aria-label")).toBe("選手からの感想");
  expect(tile!.querySelector("span.coach-tile-label")?.textContent).toBe("選手からの感想");
  expect(tile!.querySelector("span.rounded-full")).toBeNull();
  // 本体 #1014 で「履歴」を最後尾に足したので、感想はその1つ前。
  expect(tileLinks().at(-2)).toBe(tile);
  // 感想は取得しない（ホームの表示から届いた時期を推測させない）。
  expect(apiFetch.mock.calls.map((c) => c[0])).not.toContain("/v1/coach/feedback");
});

// 返却済みの添削の履歴（本体 #1014 B1）。最後尾に1つ足し、件数バッジは付けない。既存タイルの並びとバッジは変えない。
it("「履歴」タイルが最後尾にあり、/coach/history へ遷移し、バッジは付かない（#1014 B1）", async () => {
  mockAll();
  await act(async () => root.render(<CoachDashboard />));
  const tile = tileLinks().at(-1)!;
  expect(tile.getAttribute("href")).toBe("/coach/history");
  expect(tile.getAttribute("aria-label")).toBe("履歴");
  expect(tile.querySelector("span.coach-tile-label")?.textContent).toBe("履歴");
  expect(tile.querySelector("span.rounded-full")).toBeNull();
  // 既存のバッジ・点は従来どおり。
  expect(tileLabels().slice(0, 7)).toEqual([
    "対応中（1件）",
    "追加相談（1件）",
    "契約・申込み（1件）",
    "報酬・口座（要確認）",
    "プロフィール",
    "通知設定",
    "選手からの感想",
  ]);
  expect(container.querySelectorAll("nav.grid a span.rounded-full")).toHaveLength(4);
});

it("英語では History と出す（#1014）", async () => {
  flags.locale = "en";
  installEnglish(en);
  setActiveLocale("en");
  mockAll();
  await act(async () => root.render(<CoachDashboard />));
  expect(tileLinks().at(-1)?.getAttribute("aria-label")).toBe("History");
});

function statusSection() {
  return container.querySelector('section[aria-label="受付の状態"], section[aria-label="Availability"]');
}

// mockAll の上で /v1/coach/me の GET 応答だけを差し替える。
function mockMe(me: unknown) {
  mockAll();
  const base = apiFetch.getMockImplementation()!;
  apiFetch.mockImplementation((path: string, init?: { method?: string }) =>
    path === "/v1/coach/me" && init?.method !== "PATCH" ? Promise.resolve(me) : base(path, init));
}

it("今までの添削の件数をステータス欄に出す（#912 B1）", async () => {
  mockMe({ application, rating: { count: 3, average: 4.5, paused: false }, completed_review_count: 3 });
  await act(async () => root.render(<CoachDashboard />));
  expect(statusSection()?.textContent).toContain("今までの添削：3件");
});

it("0件でも件数の行は隠さない（#912 B2）", async () => {
  mockMe({ application, completed_review_count: 0 });
  await act(async () => root.render(<CoachDashboard />));
  expect(statusSection()?.textContent).toContain("今までの添削：0件");
});

it("応答に件数のキーが無いときは行を出さず、他は従来どおり（#912 B3・B5）", async () => {
  mockAll();
  await act(async () => root.render(<CoachDashboard />));
  expect(container.textContent).not.toContain("今までの添削");
  // 受付状態・タイル・バッジ・フッターは従来どおり。
  expect(statusSection()?.textContent).toContain("受付中");
  expect(container.querySelector('[role="switch"][aria-label="受付中"]')?.getAttribute("aria-checked")).toBe("true");
  expect(tileHrefs()).toHaveLength(8);
  expect(tileLabels()).toContain("対応中（1件）");
  expect(tileLabels()).toContain("追加相談（1件）");
  expect(tileLabels()).toContain("契約・申込み（1件）");
  expect(container.textContent).toContain("未払い1,050円");
});

it("英語では Reviews completed so far と出す（#912 B4）", async () => {
  flags.locale = "en";
  installEnglish(en);
  setActiveLocale("en");
  mockMe({ application, completed_review_count: 3 });
  await act(async () => root.render(<CoachDashboard />));
  expect(container.textContent).toContain("Reviews completed so far: 3");
  expect(container.textContent).not.toContain("今までの添削");
});

it("件数があっても、受付トグルの保存は従来どおりで件数の行は残る（#912 B5）", async () => {
  mockMe({ application, completed_review_count: 2 });
  const base = apiFetch.getMockImplementation()!;
  apiFetch.mockImplementation((path: string, init?: { method?: string }) =>
    path === "/v1/coach/me" && init?.method === "PATCH"
      ? Promise.resolve({ application: { ...application, on_duty: false } })
      : base(path, init));
  await act(async () => root.render(<CoachDashboard />));
  const toggle = container.querySelector<HTMLButtonElement>('[role="switch"]')!;
  await act(async () => toggle.click());
  expect(apiFetch).toHaveBeenCalledWith("/v1/coach/me", expect.objectContaining({ method: "PATCH", body: JSON.stringify({ on_duty: false }) }));
  expect(container.querySelector('[role="switch"]')?.getAttribute("aria-checked")).toBe("false");
  expect(statusSection()?.textContent).toContain("受付停止中");
  expect(statusSection()?.textContent).toContain("今までの添削：2件");
});

it("未承認・準備中の画面には件数の行を出さない（#912 B5）", async () => {
  apiFetch.mockResolvedValue({ application: { ...application, status: "pending" }, completed_review_count: 4 });
  await act(async () => root.render(<CoachDashboard />));
  expect(container.textContent).not.toContain("今までの添削");

  flags.enabled = false;
  mockMe({ application, completed_review_count: 4 });
  await renderFresh();
  expect(container.textContent).toContain("受付開始に向けて準備中です");
  expect(container.textContent).not.toContain("今までの添削");
});

// #912（件数の行）と #911（感想タイル）は同じホームで両立する。
it("件数を含む応答では、件数の行と「選手からの感想」タイルの両方が出る（#911 × #912）", async () => {
  mockMe({ application, rating: { count: 3, average: 4.5, paused: false }, completed_review_count: 3 });
  await act(async () => root.render(<CoachDashboard />));
  expect(statusSection()?.textContent).toContain("今までの添削：3件");
  const tile = tileLinks().find((a) => a.getAttribute("href") === "/coach/feedback");
  expect(tile?.getAttribute("aria-label")).toBe("選手からの感想");
  expect(tile?.querySelector("span.rounded-full")).toBeNull();
  // 最後尾は「履歴」（本体 #1014）、感想はその1つ前。
  expect(tileLinks().at(-2)).toBe(tile);
  expect(tileLabels()).toContain("対応中（1件）");
});
