/** @vitest-environment jsdom */
import { act } from "react";
import { createRoot, type Root } from "react-dom/client";
import { afterEach, beforeEach, expect, it, vi } from "vitest";
import { CoachWithdrawForm, ReviewWithdrawalNotice } from "./review-withdraw";

const apiFetch = vi.fn();
const confirm = vi.fn(async () => true);
vi.mock("@/lib/api", () => ({
  apiFetch: (...args: unknown[]) => apiFetch(...args),
  ApiRequestError: class extends Error {},
}));
vi.mock("@/components/confirm-dialog", () => ({ useConfirm: () => confirm }));

let root: Root;
let container: HTMLDivElement;
beforeEach(() => {
  (globalThis as typeof globalThis & { IS_REACT_ACT_ENVIRONMENT: boolean }).IS_REACT_ACT_ENVIRONMENT = true;
  apiFetch.mockReset();
  confirm.mockClear();
  container = document.createElement("div");
  document.body.appendChild(container);
  root = createRoot(container);
});
afterEach(async () => {
  await act(async () => root.unmount());
  container.remove();
});

it("確認してから理由を添えて辞退を送り、終わった依頼を渡す", async () => {
  const done = { id: "r1", status: "expired" };
  apiFetch.mockResolvedValue(done);
  const onWithdrawn = vi.fn();
  await act(async () => root.render(<CoachWithdrawForm requestID="r1" token="tok" onWithdrawn={onWithdrawn} />));
  const textarea = container.querySelector("textarea")!;
  await act(async () => {
    const setter = Object.getOwnPropertyDescriptor(HTMLTextAreaElement.prototype, "value")!.set!;
    setter.call(textarea, "  体調不良のため  ");
    textarea.dispatchEvent(new Event("input", { bubbles: true }));
  });
  await act(async () => {
    container.querySelector("form")!.dispatchEvent(new Event("submit", { bubbles: true, cancelable: true }));
  });
  expect(confirm).toHaveBeenCalledOnce();
  expect(apiFetch).toHaveBeenCalledWith("/v1/coach/requests/r1/withdraw", expect.objectContaining({ method: "POST", token: "tok", body: JSON.stringify({ reason: "体調不良のため" }) }));
  expect(onWithdrawn).toHaveBeenCalledWith(done);
});

it("確認で取りやめたら送らない", async () => {
  confirm.mockResolvedValueOnce(false);
  const onWithdrawn = vi.fn();
  await act(async () => root.render(<CoachWithdrawForm requestID="r1" token="tok" onWithdrawn={onWithdrawn} />));
  await act(async () => {
    container.querySelector("form")!.dispatchEvent(new Event("submit", { bubbles: true, cancelable: true }));
  });
  expect(apiFetch).not.toHaveBeenCalled();
  expect(onWithdrawn).not.toHaveBeenCalled();
});

it("依頼者には辞退で終わったことと、コーチのひとことを見せる", async () => {
  await act(async () =>
    root.render(<ReviewWithdrawalNotice withdrawal={{ withdrawn_at: "2026-11-03T12:00:00Z", from_status: "in_review", reason: "体調不良のため" }} paid />),
  );
  expect(container.textContent).toContain("担当のコーチの都合で、この依頼は終了しました");
  expect(container.textContent).toContain("コーチからのひとこと: 体調不良のため");
  expect(container.textContent).toContain("お支払いは取り消します（PayPay は返金します）");
  expect(container.textContent).toContain("別のコーチには回りません");
});
