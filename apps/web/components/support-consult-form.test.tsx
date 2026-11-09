/** @vitest-environment jsdom */
import { act } from "react";
import { createRoot, type Root } from "react-dom/client";
import { afterEach, beforeEach, expect, it, vi } from "vitest";
import { SupportConsultForm } from "./support-consult-form";
import type { PublicSupportConsult } from "@/lib/support-consult";

const apiFetch = vi.fn();
vi.mock("@/lib/api", async (original) => ({ ...await original<typeof import("@/lib/api")>(), apiFetch: (...args: unknown[]) => apiFetch(...args) }));

const consult: PublicSupportConsult = { events: ["100m"], description: "見ます", price_min_yen: null, price_max_yen: null, reception: { status: "open", resume_on: null } };
let root: Root;
let container: HTMLDivElement;
beforeEach(() => {
  (globalThis as typeof globalThis & { IS_REACT_ACT_ENVIRONMENT: boolean }).IS_REACT_ACT_ENVIRONMENT = true;
  container = document.createElement("div"); document.body.appendChild(container); root = createRoot(container);
  apiFetch.mockReset();
});
afterEach(async () => { await act(async () => root.unmount()); container.remove(); });

const render = (props: Partial<{ minor: boolean | null; consult: PublicSupportConsult }> = {}) =>
  act(async () => root.render(<SupportConsultForm coachID="c1" consult={props.consult ?? consult} token="t" minor={props.minor ?? false} onSent={() => {}} />));

it("冒頭で契約ではないこと・支払いがないことを伝え、カード情報の入力欄はない", async () => {
  await render();
  expect(container.textContent).toContain("相談と提案は契約ではありません");
  expect(container.textContent).toContain("カード情報も入力しません");
  expect(container.querySelector('input[autocomplete^="cc-"]')).toBeNull();
  expect(container.textContent).not.toContain("その他、コーチに伝えたいこと");
  expect(container.textContent).toContain("その他、コーチに聞きたいこと");
});

it("未成年には、保護者に提案を送ることと、契約には保護者の同意とお支払いが必要なことを伝える", async () => {
  await render({ minor: true });
  expect(container.textContent).toContain("契約には保護者の同意とお支払いが必要です。");
});

it("大会が決まっているとき、大会名と開催日の欄が出る。未定なら時期の目安の選択が出る", async () => {
  await render();
  const pressed = async (label: string, name: string) => {
    const group = container.querySelector(`[aria-label="${label}"]`)!;
    await act(async () => [...group.querySelectorAll("button")].find((b) => b.textContent === name)!.click());
  };
  expect(container.querySelector('input[aria-label="大会名"]')).toBeNull();
  await pressed("目標の大会は決まっていますか", "はい");
  expect(container.querySelector('input[aria-label="大会名"]')).not.toBeNull();
  expect(container.querySelector('input[aria-label="開催日"]')).not.toBeNull();
  await pressed("目標の大会は決まっていますか", "いいえ");
  expect(container.querySelector('input[aria-label="大会名"]')).toBeNull();
  expect(container.textContent).toContain("大会の時期の目安");
});

it("送信すると回答を相談として送り、サーバーの項目エラーを表示する", async () => {
  const { ApiRequestError } = await import("@/lib/api");
  apiFetch.mockRejectedValue(new ApiRequestError(422, { error: { code: "validation_error", message: "入力を確かめてください", fields: { goals: "大会での目標を選んでください" } } }));
  await render();
  await act(async () => container.querySelector("form")!.dispatchEvent(new Event("submit", { bubbles: true, cancelable: true })));
  expect(apiFetch).toHaveBeenCalledTimes(1);
  const [path, options] = apiFetch.mock.calls[0];
  expect(path).toBe("/v1/support-consults");
  const body = JSON.parse(options.body);
  expect(body.coach_id).toBe("c1");
  expect(body.answers.event).toBe("100m");
  expect(container.textContent).toContain("大会での目標を選んでください");
});

it("受付停止中は送れない", async () => {
  await render({ consult: { ...consult, reception: { status: "paused", resume_on: "2026-11-20" } } });
  expect(container.textContent).toContain("いま新しい相談を受け付けていません");
  expect((container.querySelector('button[type="submit"]') as HTMLButtonElement).disabled).toBe(true);
});
