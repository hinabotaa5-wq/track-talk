/** @vitest-environment jsdom */
import { act, type ReactNode } from "react";
import { createRoot, type Root } from "react-dom/client";
import { afterEach, beforeEach, expect, it, vi } from "vitest";
import Page from "./page";

const mocks = vi.hoisted(() => ({ api: vi.fn() }));
vi.mock("next/link", () => ({ default: ({ href, children }: { href: string; children: ReactNode }) => <a href={href}>{children}</a> }));
vi.mock("@/components/auth-provider", () => ({ useAuth: () => ({ token: "admin" }) }));
vi.mock("@/components/coaching-gate", () => ({ CoachingGate: ({ children }: { children: ReactNode }) => children }));
vi.mock("@/lib/api", async (original) => ({ ...await original<typeof import("@/lib/api")>(), apiFetch: (...args: unknown[]) => mocks.api(...args) }));
const healthy = {
  ticket_lot_ledger_mismatches: 0, paid_purchases_without_lot: 0, consumed_ticket_earnings_mismatches: 0,
  pending_ticket_refunds: 1, pending_consultation_refunds: 0, pending_support_refunds: 0, unpaid_coach_earnings: 3,
};
let container: HTMLDivElement;
let root: Root;
beforeEach(() => {
  (globalThis as typeof globalThis & { IS_REACT_ACT_ENVIRONMENT: boolean }).IS_REACT_ACT_ENVIRONMENT = true;
  mocks.api.mockReset();
  container = document.createElement("div"); document.body.appendChild(container); root = createRoot(container);
});
afterEach(async () => { await act(async () => root.unmount()); container.remove(); });

it("通常の送金待ち・返金待ちを会計の不一致として警告しない", async () => {
  mocks.api.mockResolvedValue(healthy);
  await act(async () => root.render(<Page />));
  expect(mocks.api).toHaveBeenCalledWith("/v1/admin/coaching/reconciliation", expect.objectContaining({ token: "admin" }));
  expect(container.textContent).toContain("上記の照合では不一致は見つかりませんでした");
  expect(container.querySelector('[role="alert"]')).toBeNull();
  expect(container.textContent).toContain("コーチへの送金待ち3件");
});

it("不一致を警告し、再照合の失敗時は古い結果を表示し続けない", async () => {
  mocks.api.mockResolvedValueOnce({ ...healthy, consumed_ticket_earnings_mismatches: 1 });
  await act(async () => root.render(<Page />));
  expect(container.querySelector('[role="alert"]')?.textContent).toContain("不一致があります");
  mocks.api.mockRejectedValueOnce(new Error("offline"));
  await act(async () => container.querySelector("button")!.click());
  expect(container.querySelector('[role="alert"]')?.textContent).toContain("照合できませんでした");
  expect(container.textContent).not.toContain("上記の照合では不一致は見つかりませんでした");
  expect(container.textContent).not.toContain("確認日時");
});
