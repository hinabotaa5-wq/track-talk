/** @vitest-environment jsdom */
import { act, type ReactNode } from "react";
import { createRoot, type Root } from "react-dom/client";
import { afterEach, beforeEach, expect, it, vi } from "vitest";
import { CoachServicePlans } from "./coach-service-plans";
import type { CoachingPlan } from "@/lib/types";
import type { PublicSupportConsult } from "@/lib/support-consult";
vi.mock("next/link", () => ({ default: ({ href, children }: { href: string; children: ReactNode }) => <a href={href}>{children}</a> }));
const plan: CoachingPlan = { id: "support", title: "スタートを改善する", description: "改善点とドリルを返します", kind: "single", duration_days: 7, price_yen: 2000, capacity: 2, published: true, service_type: "competition_support", video_reviews: 1, menu_updates: null, reply_formats: ["text"], start_on_first_video: false };
const consult: PublicSupportConsult = { events: ["100m"], description: "見ます", price_min_yen: 5000, price_max_yen: 12000, reception: { status: "open", resume_on: null } };
let root: Root;
let container: HTMLDivElement;
beforeEach(() => {
  (globalThis as typeof globalThis & { IS_REACT_ACT_ENVIRONMENT: boolean }).IS_REACT_ACT_ENVIRONMENT = true;
  container = document.createElement("div"); document.body.appendChild(container); root = createRoot(container);
});
afterEach(async () => { await act(async () => root.unmount()); container.remove(); });
async function render(plans: CoachingPlan[] = [plan], coachID = "c1", c: PublicSupportConsult | null = consult) {
  await act(async () => root.render(<CoachServicePlans plans={plans} coachID={coachID} consult={c} />));
}
it("単一の「サポートプラン」セクションに、受付状況・価格目安・無料相談ボタンを出す。種別の切り分けはしない", async () => {
  await render();
  expect(container.querySelectorAll("button").length).toBe(0);
  expect(container.textContent).toContain("サポートプラン");
  expect(container.textContent).toContain("受付中");
  expect(container.textContent).toContain("5,000円〜12,000円");
  expect(container.querySelector("a")?.getAttribute("href")).toBe("/coaches/c1/consult");
  expect(container.querySelector("a")?.textContent).toBe("無料相談をする");
  // 価格の決まった完成品のプランは出さない。
  expect(container.textContent).not.toContain("スタートを改善する");
});

it("対面プランは出さない", async () => {
  await render([{ ...plan, id: "in-person", title: "競技場での指導", service_type: "in_person" }]);
  expect(container.textContent).not.toContain("競技場での指導");
  expect(container.textContent).toContain("無料相談をする");
});

it("相談の入口がないコーチは受付停止の表示になり、分類のない既存プランは残す", async () => {
  await render([{ ...plan, service_type: null }], "c1", null);
  expect(container.textContent).toContain("このコーチは受付していません");
  expect(container.textContent).toContain("従来のサポートプラン");
  expect(container.textContent).toContain(plan.title);
});

it("従来の動画添削はサポートプランに表示しない", async () => {
  await render([{ ...plan, service_type: "video_review" }], "c1", null);
  expect(container.textContent).not.toContain(plan.title);
});
