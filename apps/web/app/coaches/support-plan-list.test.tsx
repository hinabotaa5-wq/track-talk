/** @vitest-environment jsdom */
import { act, type ReactNode } from "react";
import { createRoot, type Root } from "react-dom/client";
import { afterEach, beforeEach, expect, it, vi } from "vitest";

const apiFetch = vi.fn();
const flags = { list: true };
vi.mock("next/link", () => ({ default: ({ href, children }: { href: string; children: ReactNode }) => <a href={href}>{children}</a> }));
vi.mock("@/lib/api", async (original) => ({ ...await original<typeof import("@/lib/api")>(), apiFetch: (...args: unknown[]) => apiFetch(...args) }));
vi.mock("@/lib/features", async (original) => ({ ...await original<typeof import("@/lib/features")>(), get supportPlanListEnabled() { return flags.list; } }));
const { SupportPlanList } = await import("./support-plan-list");

const competition = { id: "a", nickname: "大会コーチ", photo_url: "", events: ["100m"], kinds: ["competition_support"], competition_support: { events: ["100m"], description: "", price_min_yen: 5000, price_max_yen: 12000, reception: { status: "open", resume_on: null } } };
const inPerson = { id: "b", nickname: "対面コーチ", photo_url: "", events: ["100m"], kinds: ["in_person"], in_person: { plans: [{ id: "p", title: "対面", price_yen: 8000, session_minutes: 60, venue_area: "東京都" }], min_price_yen: 8000, areas: ["東京都"], reception: { status: "paused" } } };
let root: Root;
let container: HTMLDivElement;
beforeEach(() => {
  (globalThis as typeof globalThis & { IS_REACT_ACT_ENVIRONMENT: boolean }).IS_REACT_ACT_ENVIRONMENT = true;
  container = document.createElement("div"); document.body.appendChild(container); root = createRoot(container);
  apiFetch.mockReset(); flags.list = true;
});
afterEach(async () => { await act(async () => root.unmount()); container.remove(); });
const render = async () => { await act(async () => root.render(<SupportPlanList />)); await act(async () => { await Promise.resolve(); }); };
const click = (label: string) => act(async () => { [...container.querySelectorAll("button")].find((b) => b.textContent === label)!.click(); });

it("開始前（フラグが閉じている間）は「12月開始予定」だけを出し、一覧を読みに行かない", async () => {
  flags.list = false;
  await render();
  expect(container.textContent).toContain("12月開始予定");
  expect(apiFetch).not.toHaveBeenCalled();
});

it("初期表示は「すべて」で、API の並び順のまま。大会までは受付状況と目安、対面は場所と価格を出し、人気の指標は出さない", async () => {
  apiFetch.mockResolvedValue({ coaches: [inPerson, competition] });
  await render();
  expect(apiFetch).toHaveBeenCalledWith("/v1/coaches/support-plan");
  expect(container.querySelector('[role="radio"][aria-checked="true"]')?.textContent).toBe("すべて");
  expect([...container.querySelectorAll("li")].map((li) => li.id)).toEqual(["support-coach-b", "support-coach-a"]);
  const text = container.textContent!;
  expect(text).toContain("受付中 · 目安 5,000円〜12,000円 · 無料相談できます");
  expect(text).toContain("受付停止中 · 東京都 · 8,000円");
  expect(text).not.toMatch(/★|件|ランキング|人気|残り/);
});

it("「大会まで」「対面」で絞り込める", async () => {
  apiFetch.mockResolvedValue({ coaches: [inPerson, competition] });
  await render();
  await click("大会まで");
  expect([...container.querySelectorAll("li")].map((li) => li.id)).toEqual(["support-coach-a"]);
  await click("対面");
  expect([...container.querySelectorAll("li")].map((li) => li.id)).toEqual(["support-coach-b"]);
});
