/** @vitest-environment jsdom */
import { act, type ReactNode } from "react";
import { createRoot, type Root } from "react-dom/client";
import { afterEach, beforeEach, expect, it, vi } from "vitest";
import { CoachServicePlans } from "./coach-service-plans";
import { SupportConsultEntry } from "./support-consult-entry";
import type { PublicSupportConsult } from "@/lib/support-consult";

vi.mock("next/link", () => ({ default: ({ href, children, className }: { href: string; children: ReactNode; className?: string }) => <a href={href} className={className}>{children}</a> }));

const open: PublicSupportConsult = { events: ["100m", "200m"], description: "スプリントの走り方を見ます", price_min_yen: 5000, price_max_yen: 12000, reception: { status: "open", resume_on: null } };
let root: Root;
let container: HTMLDivElement;
beforeEach(() => {
  (globalThis as typeof globalThis & { IS_REACT_ACT_ENVIRONMENT: boolean }).IS_REACT_ACT_ENVIRONMENT = true;
  container = document.createElement("div"); document.body.appendChild(container); root = createRoot(container);
});
afterEach(async () => { await act(async () => root.unmount()); container.remove(); });

it("受付中は価格の目安と無料相談の入口を出し、契約件数や残り枠は出さない", async () => {
  await act(async () => root.render(<SupportConsultEntry coachID="c1" consult={open} />));
  expect(container.textContent).toContain("受付中");
  expect(container.textContent).toContain("5,000円〜12,000円");
  expect(container.textContent).toContain("実際の価格は、相談のあとの提案で決まります");
  expect(container.querySelector("a")?.getAttribute("href")).toBe("/coaches/c1/consult");
  expect(container.querySelector("a")?.textContent).toBe("無料相談をする");
  expect(container.textContent).not.toMatch(/満枠|満員|残り|あと\d+名|件/);
});

it("受付停止中は、再開の目安は出さず、相談の入口も出さない", async () => {
  await act(async () => root.render(<SupportConsultEntry coachID="c1" consult={{ ...open, reception: { status: "paused", resume_on: "2026-11-20" } }} />));
  expect(container.textContent).toContain("受付停止中");
  expect(container.textContent).not.toContain("再開の目安");
  expect(container.querySelector("a")).toBeNull();
});

it("コーチ詳細では、大会までプランが完成品のプランではなく無料相談の入口になる", async () => {
  const plan = { id: "p", title: "旧プラン", description: "旧プランの内容", kind: "period" as const, duration_days: 30, price_yen: 8000, capacity: 3, published: true, service_type: "competition_support" as const, video_reviews: 1, menu_updates: null, reply_formats: [] as ("text")[], start_on_first_video: false };
  await act(async () => root.render(<CoachServicePlans services={["competition_support"]} plans={[plan]} coachID="c1" consult={open} />));
  const button = container.querySelector("button")!;
  expect(button.disabled).toBe(false);
  await act(async () => button.click());
  expect(container.textContent).toContain("無料相談をする");
  expect(container.textContent).not.toContain("旧プランの内容");
  expect(container.textContent).not.toContain("8,000円");
});

it("相談の入口がないコーチは、これまでどおり公開中のプランを出す", async () => {
  const plan = { id: "p", title: "旧プラン", description: "旧プランの内容", kind: "period" as const, duration_days: 30, price_yen: 8000, capacity: 3, published: true, service_type: "competition_support" as const, video_reviews: 1, menu_updates: null, reply_formats: [] as ("text")[], start_on_first_video: false };
  await act(async () => root.render(<CoachServicePlans services={["competition_support"]} plans={[plan]} coachID="c1" consult={null} />));
  await act(async () => container.querySelector("button")!.click());
  expect(container.textContent).toContain("旧プランの内容");
});
