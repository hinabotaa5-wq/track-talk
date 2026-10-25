/** @vitest-environment jsdom */
import { act, type ReactNode } from "react";
import { createRoot, type Root } from "react-dom/client";
import { afterEach, beforeEach, expect, it, vi } from "vitest";
import type { CoachApplication, CoachingPlan } from "@/lib/types";
import { CoachPlansSection } from "./coaching-plans";

const apiFetch = vi.fn();
vi.mock("next/link", () => ({ default: ({ href, children }: { href: string; children: ReactNode }) => <a href={href}>{children}</a> }));
vi.mock("@/lib/api", async (original) => ({ ...await original<typeof import("@/lib/api")>(), apiFetch: (...args: unknown[]) => apiFetch(...args) }));

const coach: CoachApplication = {
  tier: "coach", events: ["100m"], best_record: "10秒台", achievements: "全国大会出場", years: 10, sns_urls: [],
  status: "approved", applied_at: "2026-10-23T00:00:00Z", reviewed_at: null,
  on_duty: false, daily_cap: 5, photo_url: "", pledged: true, payouts_ready: true,
  service_types: ["video_review", "competition_support", "in_person"],
};
const legacyPlan: CoachingPlan = {
  id: "legacy", kind: "period", title: "以前からのプラン", description: "指導内容", duration_days: 30, price_yen: 8000,
  capacity: 3, published: true, video_reviews: 4, menu_updates: "weekly", reply_formats: ["text"], start_on_first_video: false,
  service_type: null, additional_consultation_price_yen: null,
};
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
async function render(plans: CoachingPlan[] = []) {
  apiFetch.mockImplementation(async (path: string, options?: { body?: string }) => {
    if (path.endsWith("/plans") && !options?.body) return { plans };
    if (path.endsWith("/services")) return { application: { ...coach, ...JSON.parse(options!.body!) } };
    return { plan: { ...legacyPlan, ...JSON.parse(options!.body!) } };
  });
  await act(async () => root.render(<CoachPlansSection coach={coach} token="test" onCoachChange={vi.fn()} />));
}

it("対面指導は公開して保存できる", async () => {
  await render();
  await click("プランを追加する");
  await click("対面指導");
  const checkbox = [...container.querySelectorAll<HTMLInputElement>('input[type="checkbox"]')].find((el) => el.parentElement?.textContent?.includes("公開する"));
  expect(checkbox?.disabled).toBe(false);
  expect(checkbox?.checked).toBe(true);
  await click("保存する");
  const save = apiFetch.mock.calls.find(([, options]) => options?.method === "POST");
  expect(JSON.parse(save![1].body)).toMatchObject({ service_type: "in_person", published: true });
});

it("既存プランの非公開操作で、設定済みの分類と追加相談料金を消さない", async () => {
  await render([{ ...legacyPlan, service_type: "video_review", additional_consultation_price_yen: 750 }]);
  await click("非公開にする");
  const save = apiFetch.mock.calls.find(([, options]) => options?.method === "PATCH");
  expect(JSON.parse(save![1].body)).toMatchObject({ service_type: "video_review", additional_consultation_price_yen: 750, published: false });
});

it("分類を推測せず既存プランとして表示し、新規作成時は分類を求める", async () => {
  await render([legacyPlan]);
  expect(container.textContent).toContain("従来のサポートプラン");
  await click("プランを追加する");
  await click("保存する");
  expect(container.textContent).toContain("サービスを選んでください");
  expect(apiFetch.mock.calls.some(([, options]) => options?.method === "POST")).toBe(false);
});
