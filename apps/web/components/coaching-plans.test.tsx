/** @vitest-environment jsdom */
import { act, type ReactNode } from "react";
import { createRoot, type Root } from "react-dom/client";
import { afterEach, beforeEach, expect, it, vi } from "vitest";
import type { CoachApplication, CoachingPlan } from "@/lib/types";
import { coachPledgeItems } from "@/lib/coaching-plan";
import { CoachPlansSection } from "./coaching-plans";

// サポートプラン（1ヶ月プランのみ）の画面。担当サービスの選択・対面プランの作成は廃止した（2026-11-13）。

const apiFetch = vi.fn();
vi.mock("next/link", () => ({ default: ({ href, children }: { href: string; children: ReactNode }) => <a href={href}>{children}</a> }));
vi.mock("@/lib/api", async (original) => ({ ...await original<typeof import("@/lib/api")>(), apiFetch: (...args: unknown[]) => apiFetch(...args) }));

const coach: CoachApplication = {
  tier: "coach", events: ["100m"], best_record: "10秒台", achievements: "全国大会出場", years: 10, sns_urls: [], hitokoto: "",
  status: "approved", applied_at: "2026-10-23T00:00:00Z", reviewed_at: null,
  on_duty: false, daily_cap: 5, photo_url: "", pledged: true, payouts_ready: true,
  service_types: ["video_review", "competition_support"],
};
const legacyPlan: CoachingPlan = {
  id: "legacy", kind: "period", title: "以前からのプラン", description: "指導内容", duration_days: 30, price_yen: 8000,
  capacity: 3, published: true, video_reviews: 4, menu_updates: "weekly", reply_formats: ["text"], start_on_first_video: false,
  service_type: null, additional_consultation_price_yen: null,
};
const settings = { accepting: false, events: ["100m"], description: "", price_min_yen: null, price_max_yen: null };
let root: Root;
let container: HTMLDivElement;
beforeEach(() => {
  (globalThis as typeof globalThis & { IS_REACT_ACT_ENVIRONMENT: boolean }).IS_REACT_ACT_ENVIRONMENT = true;
  container = document.createElement("div"); document.body.appendChild(container); root = createRoot(container);
  apiFetch.mockReset();
});
afterEach(async () => { await act(async () => root.unmount()); container.remove(); });
async function click(text: string) {
  const button = [...container.querySelectorAll("button")].find((el) => el.textContent === text);
  if (!button) throw new Error(`ボタンがありません: ${text}`);
  await act(async () => button.click());
}
async function render(plans: CoachingPlan[] = [], c: CoachApplication = coach, onCoachChange = vi.fn()) {
  apiFetch.mockImplementation(async (path: string, options?: { method?: string; body?: string }) => {
    if (path.endsWith("/plans") && !options?.body) return { plans };
    if (path.endsWith("/support-consult")) return { settings: options?.body ? JSON.parse(options.body) : settings, disclosure_approved: true };
    if (path.endsWith("/services")) return { application: { ...c, ...JSON.parse(options!.body!) } };
    return { plan: { ...legacyPlan, ...JSON.parse(options!.body!) } };
  });
  await act(async () => root.render(<CoachPlansSection coach={c} token="test" onCoachChange={onCoachChange} />));
}

it("冒頭は1行の説明と受付設定のカードだけで、担当サービス・対面プラン・プラン作成の導線を出さない", async () => {
  await render();
  const text = container.textContent ?? "";
  expect(text).toContain("1ヶ月プランの受付設定です。");
  expect(container.querySelector("h2")?.textContent).toBe("受付設定");
  expect(text).toContain("受付中にすると、コーチ一覧に出て相談を受け付けます。");
  expect(text).not.toContain("担当するサービス");
  expect(text).not.toContain("対面");
  expect(text).not.toContain("プランを追加する");
  expect(text).not.toContain("動画添削はプランとして出品しません");
  expect(text).not.toContain("カレンダー");
  // 「サポートプラン」の見出しは画面タイトルに一本化し、中では繰り返さない。
  expect([...container.querySelectorAll("h2, h3")].map((h) => h.textContent)).not.toContain("サポートプラン");
});

it("受付中／停止は既存の受付設定 API にそのまま保存する", async () => {
  await render();
  const box = [...container.querySelectorAll<HTMLInputElement>('input[type="checkbox"]')].find((el) => el.parentElement?.textContent?.includes("受付中にする"));
  await act(async () => box!.click());
  await click("保存する");
  const put = apiFetch.mock.calls.find(([path, options]) => path === "/v1/coach/me/support-consult" && options?.method === "PUT");
  expect(JSON.parse(put![1].body)).toMatchObject({ accepting: true, events: ["100m"] });
  expect(apiFetch.mock.calls.some(([path]) => path === "/v1/coach/me/services")).toBe(false);
});

it("1ヶ月プランを担当サービスに登録していないコーチが受付中で保存すると、既存の担当サービス API で登録してから保存する", async () => {
  const onCoachChange = vi.fn();
  await render([], { ...coach, service_types: ["video_review"] }, onCoachChange);
  const box = [...container.querySelectorAll<HTMLInputElement>('input[type="checkbox"]')].find((el) => el.parentElement?.textContent?.includes("受付中にする"));
  expect(box?.disabled).toBe(false);
  await act(async () => box!.click());
  await click("保存する");
  const paths = apiFetch.mock.calls.filter(([, options]) => options?.method).map(([path, options]) => `${options.method} ${path}`);
  expect(paths).toEqual(["PATCH /v1/coach/me/services", "PUT /v1/coach/me/support-consult"]);
  const patch = apiFetch.mock.calls.find(([path]) => path === "/v1/coach/me/services");
  expect(JSON.parse(patch![1].body)).toEqual({ service_types: ["video_review", "competition_support"] });
  expect(onCoachChange).toHaveBeenCalled();
});

it("誓約前は短縮した約束を全項目出し、同意してから誓約できる", async () => {
  await render([], { ...coach, pledged: false });
  const text = container.textContent ?? "";
  expect(text).toContain("約束してください（利用者の多くは中高生です）");
  expect(coachPledgeItems).toHaveLength(6);
  for (const item of coachPledgeItems) expect(text).toContain(item);
  expect(text).toContain("外部の連絡先（LINE・SNS・電話・メール）は教えない・聞かない");
  expect(text).toContain("利用者と直接会うのは、TrackTalkで購入・予約した場合だけ");
  const pledge = [...container.querySelectorAll("button")].find((el) => el.textContent === "誓約する");
  expect(pledge?.disabled).toBe(true);
  expect(container.querySelector('section[aria-label="受付設定"]')).toBeNull();
});

it("公開中のまま残っている以前のプランは、非公開にだけできる（設定済みの分類と追加相談料金は消さない）", async () => {
  await render([{ ...legacyPlan, service_type: "in_person", additional_consultation_price_yen: 750 }]);
  expect(container.textContent).toContain("公開中の以前のプラン");
  expect([...container.querySelectorAll("button")].some((el) => el.textContent === "直す")).toBe(false);
  await click("非公開にする");
  const save = apiFetch.mock.calls.find(([, options]) => options?.method === "PATCH");
  expect(JSON.parse(save![1].body)).toMatchObject({ service_type: "in_person", additional_consultation_price_yen: 750, published: false });
});

it("非公開・1回プランのプランしかなければ、以前のプランの欄は出さない", async () => {
  await render([{ ...legacyPlan, published: false }, { ...legacyPlan, id: "v", service_type: "video_review" }]);
  expect(container.textContent).not.toContain("公開中の以前のプラン");
});
