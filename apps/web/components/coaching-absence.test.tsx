/** @vitest-environment jsdom */
import { act } from "react";
import { createRoot, type Root } from "react-dom/client";
import { afterEach, beforeEach, expect, it, vi } from "vitest";
import type { CoachingContract } from "@/lib/types";
import { CoachingAbsence, type Absence } from "./coaching-absence";

const apiFetch = vi.fn();
vi.mock("@/lib/api", async (original) => ({ ...(await original<typeof import("@/lib/api")>()), apiFetch: (...args: unknown[]) => apiFetch(...args) }));

const declared: Absence = {
  id: "a1", status: "declared", absent_from: "2026-11-20", return_on: "2026-11-23", absent_days: 3, reason: "体調不良のため", minor: false, price_yen: 9000,
  original_ends_at: "2026-11-30T15:00:00Z", proposed_ends_at: "2026-12-03T15:00:00Z", refund_estimate_yen: 8100,
  player_consented_at: null, guardian_consented_at: null, coach_consented_at: null, agreed_at: null, agreed_original_ends_at: null, agreed_new_ends_at: null,
};
const contract = { id: "c1", status: "active", role: "requester" } as unknown as CoachingContract;

let root: Root;
let container: HTMLDivElement;
beforeEach(() => {
  (globalThis as typeof globalThis & { IS_REACT_ACT_ENVIRONMENT: boolean }).IS_REACT_ACT_ENVIRONMENT = true;
  container = document.createElement("div");
  document.body.appendChild(container);
  root = createRoot(container);
  apiFetch.mockReset();
});
afterEach(async () => {
  await act(async () => root.unmount());
  container.remove();
});

async function render(c: CoachingContract, list: { absences: Absence[]; eligible: boolean }, guardianToken?: string) {
  apiFetch.mockResolvedValue({ ...list, max_total_days: 7 });
  await act(async () => root.render(<CoachingAbsence contract={c} token={guardianToken ? null : "token"} guardianToken={guardianToken} />));
}

it("選手への提示に、延長案（新しい終了日）と終了案（返金概算額）の両方と、いつでも終了できることが入る", async () => {
  await render(contract, { absences: [declared], eligible: false });
  const text = container.textContent ?? "";
  expect(text).toContain("案1: 契約を延長する");
  expect(text).toContain("追加料金はなく");
  expect(text).toContain("案2: 延長せずに終了する");
  expect(text).toContain("8,100円");
  expect(text).toContain("いつでも契約を終了できます");
  expect(text).toContain("合計7日まで");
});

it("「延長する」を押したときだけ同意を送る。返事をしないだけでは何も送らない", async () => {
  await render(contract, { absences: [declared], eligible: false });
  expect(apiFetch.mock.calls.filter(([, o]) => o?.method === "POST")).toHaveLength(0);
  const button = [...container.querySelectorAll("button")].find((b) => b.textContent === "延長する")!;
  await act(async () => button.click());
  const posts = apiFetch.mock.calls.filter(([, o]) => o?.method === "POST");
  expect(posts[0][0]).toBe("/v1/coaching/contracts/c1/absences/a1/consent");
  expect(JSON.parse(posts[0][1].body)).toEqual({ agree: true });
});

it("保護者は自分のリンクの API で同意する。未成年のときは保護者の同意も必要と出る", async () => {
  await render({ ...contract, role: "guardian" } as CoachingContract, { absences: [{ ...declared, minor: true }], eligible: false }, "gtoken");
  expect(container.textContent).toContain("保護者の同意も必要です");
  const button = [...container.querySelectorAll("button")].find((b) => b.textContent === "延長する")!;
  await act(async () => button.click());
  const posts = apiFetch.mock.calls.filter(([, o]) => o?.method === "POST");
  expect(posts[0][0]).toBe("/v1/guardian/gtoken/contracts/c1/absences/a1/consent");
});

it("合意が成立した延長は、元の終了日・新しい終了日とともに契約の画面に残る", async () => {
  const agreed: Absence = { ...declared, status: "agreed", agreed_at: "2026-11-18T01:00:00Z", agreed_original_ends_at: declared.original_ends_at, agreed_new_ends_at: declared.proposed_ends_at };
  await render(contract, { absences: [agreed], eligible: false });
  expect(container.textContent).toContain("不在日の延長に合意しています");
  expect(container.textContent).toContain("価格と内容は変わりません");
});

it("コーチは、対象の契約で対応できない日を申告できる。選手には申告ボタンを出さない", async () => {
  await render({ ...contract, role: "coach" } as CoachingContract, { absences: [], eligible: true });
  expect(container.textContent).toContain("対応できない日を申告する");
  await act(async () => root.unmount());
  root = createRoot(container);
  await render(contract, { absences: [], eligible: true });
  expect(container.textContent).toBe("");
});
