/** @vitest-environment jsdom */
import { act } from "react";
import { createRoot, type Root } from "react-dom/client";
import { afterEach, beforeEach, expect, it, vi } from "vitest";
import { installEnglish, setActiveLocale } from "@/lib/i18n";
import { en } from "@/lib/i18n-en";
import type { CoachingContract } from "@/lib/types";

const apiFetch = vi.fn();
let contractFixture: unknown;
vi.mock("next/navigation", () => ({
  useParams: () => ({ id: "c1" }),
  useSearchParams: () => ({ get: () => null }),
  useRouter: () => ({ replace: vi.fn() }),
}));
vi.mock("@/components/auth-provider", () => ({
  useAuth: () => ({ session: { user: { id: "u1" } }, token: "t", loading: false }),
}));
vi.mock("@/lib/api", async (original) => ({
  ...(await original<typeof import("@/lib/api")>()),
  apiFetch: (...args: unknown[]) => apiFetch(...args),
}));

const Page = (await import("./page")).default;

const base = {
  id: "c1",
  status: "applied",
  role: "requester",
  minor: false,
  payment_required: true,
  respond_by: "2026-12-05T00:00:00Z",
  start_by: "2026-12-10T00:00:00Z",
  starts_at: "2026-12-01T00:00:00Z",
  ends_at: "2026-12-31T00:00:00Z",
  plan_title: "1ヶ月プラン",
  plan_kind: "period",
  duration_days: 30,
  price_yen: 20000,
  plan_description: "サービス内容X",
  event: "100m",
  goal: "目標X",
  current_record: "記録X",
  message: "相談X",
  service_type: "competition_support",
  fulfillment_version: 0,
  coach: { id: "coach1", nickname: "コーチA", photo_url: "https://example.com/a.jpg" },
  requester: { nickname: "選手B" },
  payment: null,
  messages_purged_at: null,
  messages_view_until: null,
  clock_started_at: null,
  video_reviews: 0,
  reply_formats: [],
  menu_updates: null,
  start_on_first_video: false,
  created_at: "2026-11-01T00:00:00Z",
  responded_at: null,
  suspended_at: null,
} as unknown as CoachingContract;

let root: Root;
let container: HTMLDivElement;
beforeEach(() => {
  (globalThis as typeof globalThis & { IS_REACT_ACT_ENVIRONMENT: boolean }).IS_REACT_ACT_ENVIRONMENT = true;
  container = document.createElement("div");
  document.body.appendChild(container);
  root = createRoot(container);
  apiFetch.mockReset();
  apiFetch.mockImplementation(async () => ({ contract: contractFixture }));
});
afterEach(async () => {
  await act(async () => root.unmount());
  container.remove();
  setActiveLocale("ja");
});
const render = async (c: CoachingContract) => {
  contractFixture = c;
  await act(async () => root.render(<Page />));
  await act(async () => {
    await Promise.resolve();
    await Promise.resolve();
  });
};

it("細目9: 選手表示ではヘッダーにコーチのアバター・名前・状態が出て、戻る先は一覧で重複しない", async () => {
  await render(base);
  const h1 = container.querySelector("h1");
  expect(h1?.textContent).toBe("コーチA");
  const header = container.querySelector("header");
  expect(header?.className).toContain("sticky");
  expect(header?.querySelector("img")).not.toBeNull();
  expect(header?.textContent).toContain("コーチの返事待ち");
  const back = container.querySelector('a[aria-label="申し込みの一覧へ"]') as HTMLAnchorElement;
  expect(back).not.toBeNull();
  expect(back.getAttribute("href")).toBe("/me/coaching/personal");
  expect(back.textContent).toContain("‹");
  expect(container.querySelectorAll('a[href="/me/coaching/personal"]')).toHaveLength(1);
  expect(container.textContent).not.toContain("パーソナル指導");
});

it("細目9: コーチ表示では「{選手名}さん」と頭文字アバターが出て、戻る先はコーチ画面", async () => {
  await render({ ...base, role: "coach" } as CoachingContract);
  expect(container.querySelector("h1")?.textContent).toBe("選手Bさん");
  const back = container.querySelector('a[aria-label="コーチ画面へ"]') as HTMLAnchorElement;
  expect(back.getAttribute("href")).toBe("/coach");
  expect(container.querySelectorAll('a[href="/coach"]')).toHaveLength(1);
  const header = container.querySelector("header");
  expect(header?.querySelector("img")).toBeNull();
  expect(header?.textContent).toContain("選");
});

it("細目10: 申し込み内容は初期で閉じ、中にプラン等と期間が入り、期限案内は外に出る", async () => {
  await render(base);
  const details = container.querySelector("details");
  expect(details).not.toBeNull();
  expect(details!.hasAttribute("open")).toBe(false);
  expect(details!.querySelector("summary")?.textContent).toBe("申し込み内容");
  const inside = details!.textContent ?? "";
  for (const s of ["1ヶ月プラン", "100m", "目標X", "記録X", "相談X", "期間:"]) expect(inside).toContain(s);
  const section = container.querySelector("section");
  expect(section?.textContent).toContain("までにコーチから返事が届きます。");
  expect(details!.textContent).not.toContain("までにコーチから返事が届きます。");
});

it("細目10: accepted の開始待ち案内とお支払い行は折りたたみの外", async () => {
  await render({
    ...base,
    status: "accepted",
    payment: { status: "paid", amount_yen: 20000, refunded_yen: 0, paid_at: "2026-11-20T00:00:00Z", refunded_at: null },
  } as unknown as CoachingContract);
  const details = container.querySelector("details");
  expect(details!.textContent).not.toContain("お支払い 20,000円");
  expect(container.textContent).toContain("お支払い");
  expect(details!.textContent).not.toContain("コーチが引き受けました");
  expect(container.textContent).toContain("コーチが引き受けました");
});

it("細目11: 英語では折りたたみ見出しが Application details になる", async () => {
  installEnglish(en);
  setActiveLocale("en");
  await render(base);
  expect(container.querySelector("summary")?.textContent).toBe("Application details");
  expect(container.textContent).not.toContain("申し込み内容");
});
