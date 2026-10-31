/** @vitest-environment jsdom */
import { act, type ReactNode } from "react";
import { createRoot, type Root } from "react-dom/client";
import { afterEach, beforeEach, expect, it, vi } from "vitest";
import Page from "./page";

const mocks = vi.hoisted(() => ({ api: vi.fn(), confirm: vi.fn() }));
vi.mock("next/link", () => ({ default: ({ href, children }: { href: string; children: ReactNode }) => <a href={href}>{children}</a> }));
vi.mock("@/components/auth-provider", () => ({ useAuth: () => ({ token: "admin" }) }));
vi.mock("@/components/coaching-gate", () => ({ CoachingGate: ({ children }: { children: ReactNode }) => children }));
vi.mock("@/components/confirm-dialog", () => ({ useConfirm: () => mocks.confirm }));
vi.mock("@/lib/api", async original => ({ ...await original<typeof import("@/lib/api")>(), apiFetch: (...args: unknown[]) => mocks.api(...args) }));
const data = {
 payments: [{ id: "payment-1", source: "review", amount_yen: 1500, status: "captured", created_at: "2026-10-31T00:00:00Z" }],
 refunds: [{ id: "refund-1", payment_id: "older-payment", source: "review", amount_yen: 1500, reason: "確認済み", admin_id: "admin-id", origin: "admin", status: "succeeded", created_at: "2026-10-31T00:00:00Z" }],
 adjustments: [{ id: "adj-1", coach_id: "coach-1", refund_id: "refund-1", amount_yen: 1050, applied_yen: 400, remaining_yen: 650 }],
};
let container: HTMLDivElement;
let root: Root;
beforeEach(() => {
 (globalThis as typeof globalThis & { IS_REACT_ACT_ENVIRONMENT: boolean }).IS_REACT_ACT_ENVIRONMENT = true;
 mocks.api.mockReset(); mocks.confirm.mockReset();
 container = document.createElement("div"); document.body.appendChild(container); root = createRoot(container);
});
afterEach(async () => { await act(async () => root.unmount()); container.remove(); });

it("返金理由が必要で、確認後に支払い単位の全額返金を送信する", async () => {
 mocks.api.mockResolvedValue(data); mocks.confirm.mockResolvedValue(true);
 await act(async () => root.render(<Page />));
 const button = Array.from(container.querySelectorAll("button")).find(b => b.textContent === "全額返金する")!;
 expect(button.disabled).toBe(true);
 const input = container.querySelector("input")!;
 await act(async () => {
  Object.getOwnPropertyDescriptor(HTMLInputElement.prototype, "value")!.set!.call(input, "提供内容を確認して返金");
  input.dispatchEvent(new Event("input", { bubbles: true }));
 });
 expect(button.disabled).toBe(false);
 mocks.api.mockResolvedValueOnce({ message: "受け付けました" }).mockResolvedValueOnce(data);
 await act(async () => button.click());
 expect(mocks.confirm).toHaveBeenCalledWith(expect.objectContaining({ title: "全額返金しますか？" }));
 expect(mocks.api).toHaveBeenCalledWith("/v1/admin/coaching/payments/payment-1/refund", expect.objectContaining({ method: "POST", token: "admin", body: JSON.stringify({ source: "review", reason: "提供内容を確認して返金" }) }));
 expect(container.querySelector('[role="status"]')?.textContent).toContain("受け付けました");
 expect(container.textContent).toContain("未適用 650円");
 expect(container.textContent).toContain("発生元の返金：refund-1");
});

it("権限エラーの場合は監査記録や返金ボタンを表示しない", async () => {
 mocks.api.mockRejectedValue(new Error("forbidden"));
 await act(async () => root.render(<Page />));
 expect(container.querySelector('[role="alert"]')).not.toBeNull();
 expect(container.querySelector("input")).toBeNull();
 expect(container.textContent).not.toContain("admin-id");
});
