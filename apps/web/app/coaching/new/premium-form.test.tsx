/** @vitest-environment jsdom */
import { act, type ReactNode } from "react";
import { createRoot, type Root } from "react-dom/client";
import { afterEach, beforeEach, expect, it, vi } from "vitest";

// プレミアムの申込（本体 #1053、補足 7231984993・補足2 7231991592）。動画をテーマごとのグループに分けて合計6本まで送る。
// 意識・目的の欄は全グループにあり、この時のタイム・風速・コンディションはレース全体のグループの動画にだけ出る。
const apiFetch = vi.fn();
const push = vi.fn();
vi.mock("next/navigation", () => ({ useRouter: () => ({ replace: vi.fn(), push }) }));
vi.mock("next/link", () => ({ default: ({ href, children, ...props }: { href: string; children: ReactNode }) => <a href={href} {...props}>{children}</a> }));
vi.mock("@/lib/api", async (original) => ({ ...await original<typeof import("@/lib/api")>(), apiFetch: (...args: unknown[]) => apiFetch(...args) }));
vi.mock("@/components/auth-provider", () => ({
  useAuth: () => ({ session: { user: { id: "u1" } }, token: "tok", loading: false, profile: { purchase_notice_required: false } }),
}));
vi.mock("@/lib/review", async (original) => ({ ...await original<typeof import("@/lib/review")>(), videoDuration: async () => 10 }));
vi.mock("@/lib/cloudinary-upload", async (original) => ({
  ...await original<typeof import("@/lib/cloudinary-upload")>(),
  uploadVideoToCloudinary: async (_file: File, signed: { public_id: string }) => ({ public_id: signed.public_id }),
}));
const { PremiumRequestForm } = await import("./premium-form");

let root: Root;
let container: HTMLDivElement;
let uploads = 0;
const created: Record<string, unknown>[] = [];
beforeEach(() => {
  (globalThis as typeof globalThis & { IS_REACT_ACT_ENVIRONMENT: boolean }).IS_REACT_ACT_ENVIRONMENT = true;
  container = document.createElement("div");
  document.body.appendChild(container);
  root = createRoot(container);
  uploads = 0;
  created.length = 0;
  push.mockReset();
  window.history.replaceState(null, "", "/coaching/new?plan=premium&coach=c1&pay=card");
  apiFetch.mockReset();
  apiFetch.mockImplementation(async (path: string, options?: { method?: string; body?: string }) => {
    if (path.startsWith("/v1/coaching/nominees")) {
      return { coaches: [{ id: "c1", nickname: "テストコーチ", photo_url: "", events: ["100m"], years: 1, best_record: "", achievements: "", premium_price_yen: 8000 }] };
    }
    if (path.startsWith("/v1/coaching/estimate")) return { payments: true, due_at: "2026-11-29T12:00:00+09:00" };
    if (path === "/v1/me/coaching/tickets") return { coupons: { count: 0 } };
    if (path.startsWith("/v1/uploads/coaching-video")) return { public_id: `coaching/v${++uploads}` };
    if (path === "/v1/coaching/requests" && options?.method === "POST") {
      created.push(JSON.parse(options.body ?? "{}"));
      return { id: "r1" };
    }
    throw new Error(`unexpected ${path}`);
  });
});
afterEach(async () => {
  await act(async () => root.unmount());
  container.remove();
});

async function flush() {
  for (let i = 0; i < 4; i++) await act(async () => { await Promise.resolve(); });
}

async function render() {
  await act(async () => root.render(<PremiumRequestForm />));
  await flush();
}

const button = (text: string) => {
  const found = [...container.querySelectorAll("button")].find((b) => b.textContent === text);
  if (!found) throw new Error(`button not found: ${text}`);
  return found as HTMLButtonElement;
};
const groups = () => [...container.querySelectorAll('[data-testid="premium-group"]')] as HTMLElement[];

async function addGroup(label: string) {
  await act(async () => button(`＋${label}`).click());
}

async function addVideo(group: HTMLElement, name: string) {
  const input = group.querySelector('input[type="file"]') as HTMLInputElement;
  Object.defineProperty(input, "files", { configurable: true, value: [new File(["x"], name, { type: "video/mp4" })] });
  await act(async () => { input.dispatchEvent(new Event("change", { bubbles: true })); });
  await flush();
}

async function type(el: HTMLInputElement | HTMLTextAreaElement, value: string) {
  const proto = el instanceof HTMLTextAreaElement ? HTMLTextAreaElement.prototype : HTMLInputElement.prototype;
  await act(async () => {
    Object.getOwnPropertyDescriptor(proto, "value")!.set!.call(el, value);
    el.dispatchEvent(new Event("input", { bubbles: true }));
  });
}

it("グループはテンプレ5つから作れ、同じテンプレの2つ目は番号付きのテーマ名になる。意識・目的は全グループに出て、タイム等はレース全体の動画だけ", async () => {
  await render();
  const templates = [...container.querySelectorAll('[data-testid="premium-add-group"] button')].map((b) => b.textContent);
  expect(templates).toEqual(["＋レース全体", "＋スタート", "＋後半", "＋練習", "＋その他"]);
  await addGroup("レース全体");
  await addGroup("練習");
  await addGroup("練習");
  expect(groups().map((g) => (g.querySelector('input[name="title"]') as HTMLInputElement).value)).toEqual(["レース全体", "練習", "練習2"]);
  for (const g of groups()) expect(g.querySelector('textarea[name="focus_note"]')).not.toBeNull();
  await addVideo(groups()[0], "race.mp4");
  await addVideo(groups()[1], "drill.mp4");
  expect(groups()[0].querySelector('input[name="race_time"]')).not.toBeNull();
  expect(groups()[0].querySelector('input[name="wind_speed"]')).not.toBeNull();
  expect(groups()[0].querySelector('textarea[name="body_condition"]')).not.toBeNull();
  expect(groups()[1].querySelector('input[name="race_time"]')).toBeNull();
});

it("動画のないグループがあると確認へ進めず、外せば送れる。送る形はグループごとの意識・目的とレースの入力（T2・T3・G3）", async () => {
  await render();
  await addGroup("レース全体");
  await addGroup("練習");
  await addGroup("その他");
  await addVideo(groups()[0], "race.mp4");
  await addVideo(groups()[1], "drill1.mp4");
  await addVideo(groups()[1], "drill2.mp4");
  await type(groups()[0].querySelector('input[name="race_time"]') as HTMLInputElement, "11秒02");
  await type(groups()[0].querySelector('textarea[name="focus_note"]') as HTMLTextAreaElement, "後半の失速を減らしたい");
  await type(groups()[1].querySelector('input[name="title"]') as HTMLInputElement, "ミニハードル");
  await type(groups()[1].querySelector('textarea[name="focus_note"]') as HTMLTextAreaElement, "接地を体の真下に");
  await act(async () => button("申込み内容を確認する").click());
  expect(container.textContent).toContain("動画が入っていないグループがあります");
  await act(async () => (groups()[2].querySelector("button") as HTMLButtonElement).click());
  await act(async () => button("申込み内容を確認する").click());
  expect(container.textContent).toContain("グループごとの動画");
  expect(container.textContent).toContain("レース全体 1本、ミニハードル 2本");
  await act(async () => button("確定してお支払いへ").click());
  await flush();
  expect(created).toHaveLength(1);
  expect(created[0].plan).toBe("premium");
  expect(created[0].videos).toBeUndefined();
  expect(created[0].groups).toEqual([
    { kind: "race", title: "レース全体", focus_note: "後半の失速を減らしたい", videos: [{ video_public_id: "coaching/v1", race_time: "11秒02", wind_speed: null, body_condition: null }] },
    { kind: "practice", title: "ミニハードル", focus_note: "接地を体の真下に", videos: [{ video_public_id: "coaching/v2" }, { video_public_id: "coaching/v3" }] },
  ]);
});

it("合計6本で追加の入口が消える。1つのグループに6本入れられる（G4）", async () => {
  await render();
  await addGroup("練習");
  for (let i = 1; i <= 6; i++) await addVideo(groups()[0], `drill${i}.mp4`);
  expect(groups()[0].querySelectorAll('[data-testid="premium-video"]')).toHaveLength(6);
  expect(container.textContent).toContain("動画（6/6本・1本60秒まで）");
  expect(container.querySelector('input[type="file"]')).toBeNull();
  expect(container.querySelector('[data-testid="premium-add-group"]')).toBeNull();
});
