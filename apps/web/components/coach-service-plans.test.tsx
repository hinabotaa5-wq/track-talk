/** @vitest-environment jsdom */
import { act, type ReactNode } from "react";
import { createRoot, type Root } from "react-dom/client";
import { afterEach, beforeEach, expect, it, vi } from "vitest";
import { CoachServicePlans } from "./coach-service-plans";
import type { CoachingPlan, CoachingServiceType } from "@/lib/types";
vi.mock("next/link", () => ({ default: ({ href, children }: { href: string; children: ReactNode }) => <a href={href}>{children}</a> }));
const plan: CoachingPlan = { id: "support", title: "スタートを改善する", description: "改善点とドリルを返します", kind: "single", duration_days: 7, price_yen: 2000, capacity: 2, published: true, service_type: "competition_support", video_reviews: 1, menu_updates: null, reply_formats: ["text"], start_on_first_video: false };
let root: Root;
let container: HTMLDivElement;
beforeEach(() => {
  (globalThis as typeof globalThis & { IS_REACT_ACT_ENVIRONMENT: boolean }).IS_REACT_ACT_ENVIRONMENT = true;
  container = document.createElement("div"); document.body.appendChild(container); root = createRoot(container);
});
afterEach(async () => { await act(async () => root.unmount()); container.remove(); });
async function render(plans = [plan], services: CoachingServiceType[] = ["competition_support"]) {
  await act(async () => root.render(<CoachServicePlans plans={plans} services={services} />));
}
it("最初は大会と対面の2択だけを表示し、受付していないサービスは選べない", async () => {
  await render();
  const buttons = [...container.querySelectorAll("button")];
  expect(buttons).toHaveLength(2);
  expect(buttons.map((button) => button.disabled)).toEqual([false, true]);
  expect(buttons[0].textContent).toContain("大会までプラン");
  expect(container.textContent).not.toContain(plan.description);
  await act(async () => buttons[0].click());
  expect(buttons[0].getAttribute("aria-expanded")).toBe("true");
  expect(container.textContent).toContain(plan.description);
  expect(container.querySelector("a")?.getAttribute("href")).toBe("/coaching/personal/plans/support");
});
it("別の種類へ切り替えると選択したプランだけを表示する", async () => {
  await render([plan, { ...plan, id: "in-person", title: "競技場での指導", service_type: "in_person" }], ["competition_support", "in_person"]);
  const buttons = [...container.querySelectorAll("button")];
  await act(async () => buttons[0].click());
  await act(async () => buttons[1].click());
  expect(container.textContent).toContain("競技場での指導");
  expect(container.textContent).not.toContain(plan.title);
});
it("公開プランがないサービスは無効にし、分類のない既存プランは残す", async () => {
  await render([{ ...plan, service_type: null }], ["video_review"]);
  expect([...container.querySelectorAll("button")].every((button) => button.disabled)).toBe(true);
  expect(container.textContent).toContain("従来のサポートプラン");
  expect(container.textContent).toContain(plan.title);
});
it("満員でも内容は確認でき、申し込みはできない", async () => {
  await render([{ ...plan, full: true }]);
  await act(async () => container.querySelector("button")!.click());
  expect(container.textContent).toContain("いまは満員です");
  expect(container.querySelector("a")).toBeNull();
});

it("従来の動画添削はサポートプランに表示しない", async () => {
 await render([{...plan,service_type:"video_review"}], ["video_review"]);
 expect(container.textContent).not.toContain(plan.title);
 expect([...container.querySelectorAll("button")].some(b=>b.textContent?.includes("動画添削"))).toBe(false);
});
