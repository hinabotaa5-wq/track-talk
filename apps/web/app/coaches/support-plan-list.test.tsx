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
const paused = { id: "b", nickname: "停止中コーチ", photo_url: "", events: ["200m"], kinds: ["competition_support"], competition_support: { events: ["200m"], description: "", price_min_yen: null, price_max_yen: null, reception: { status: "paused", resume_on: null } } };
let root: Root;
let container: HTMLDivElement;
beforeEach(() => {
  (globalThis as typeof globalThis & { IS_REACT_ACT_ENVIRONMENT: boolean }).IS_REACT_ACT_ENVIRONMENT = true;
  container = document.createElement("div"); document.body.appendChild(container); root = createRoot(container);
  apiFetch.mockReset(); flags.list = true;
});
afterEach(async () => { await act(async () => root.unmount()); container.remove(); });
const render = async () => { await act(async () => root.render(<SupportPlanList />)); await act(async () => { await Promise.resolve(); }); };
const ids = () => [...container.querySelectorAll("li")].map((li) => li.id);

it("開始前（フラグが閉じている間）は「12月開始予定」だけを出し、一覧を読みに行かない", async () => {
  flags.list = false;
  await render();
  expect(container.textContent).toContain("12月開始予定");
  expect(apiFetch).not.toHaveBeenCalled();
});

it("種別の絞り込みは置かず、初期表示は対象コーチ全員。1ヶ月プランの受付状況と目安を出し、人気の指標は出さない", async () => {
  apiFetch.mockResolvedValue({ coaches: [competition, paused] });
  await render();
  expect(apiFetch).toHaveBeenCalledWith("/v1/coaches/support-plan");
  // 種別の絞り込み（すべて／大会まで／対面）はない。種目チップだけ。
  expect(container.querySelector('[aria-label="絞り込み"]')).toBeNull();
  expect(ids()).toEqual(["support-coach-a", "support-coach-b"]);
  const text = container.textContent!;
  expect(text).toContain("1ヶ月プラン");
  expect(text).toContain("受付中 · 目安 5,000円〜12,000円 · 無料相談できます");
  expect(text).toContain("受付停止中");
  expect(text).not.toMatch(/★|件|ランキング|人気|残り/);
});

const jump = { ...paused, id: "c", nickname: "跳躍コーチ", events: ["long-jump", "triple-jump"] };
const radios = (group: string) => [...container.querySelectorAll(`[role="radiogroup"][aria-label="${group}"] [role="radio"]`)];
const pick = (group: string, label: string) => act(async () => { (radios(group).find((r) => r.textContent?.replace("✓", "") === label) as HTMLButtonElement).click(); });

it("種目のチップだけが効き、初期は「すべて」", async () => {
  apiFetch.mockResolvedValue({ coaches: [competition, paused, jump] });
  await render();
  const groups = [...container.querySelectorAll('[role="radiogroup"]')].map((g) => g.getAttribute("aria-label"));
  expect(groups).toEqual(["種目で絞り込む"]);
  expect(radios("種目で絞り込む").find((r) => r.getAttribute("aria-checked") === "true")?.textContent).toBe("✓すべて");
  expect(ids()).toEqual(["support-coach-a", "support-coach-b", "support-coach-c"]);
});

it("種目の絞り込みで絞れ、並びは API の順のまま", async () => {
  apiFetch.mockResolvedValue({ coaches: [competition, paused, jump] });
  await render();
  await pick("種目で絞り込む", "100m");
  expect(ids()).toEqual(["support-coach-a"]);
  await pick("種目で絞り込む", "走幅跳");
  expect(ids()).toEqual(["support-coach-c"]);
});

it("該当0人の種目では空状態と「すべて」へ戻る導線を出す", async () => {
  apiFetch.mockResolvedValue({ coaches: [competition, paused] });
  await render();
  await pick("種目で絞り込む", "やり投");
  expect(ids()).toEqual([]);
  expect(container.textContent).toContain("この種目のコーチはまだいません");
  await act(async () => { [...container.querySelectorAll("button")].find((b) => b.textContent === "すべての種目のコーチを見る")!.click(); });
  expect(radios("種目で絞り込む").find((r) => r.getAttribute("aria-checked") === "true")?.textContent).toBe("✓すべて");
  expect(ids()).toEqual(["support-coach-a", "support-coach-b"]);
});
