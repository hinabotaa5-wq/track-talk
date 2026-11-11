/** @vitest-environment jsdom */
import { act, type ReactNode } from "react";
import { createRoot, type Root } from "react-dom/client";
import { afterEach, beforeEach, expect, it, vi } from "vitest";
import Page from "./page";

// 合成データのみ。実在の個人情報は使わない。
const identity = { legal_name: "合成太郎", address: "合成県合成市1-2-3", phone: "090-0000-0000" };
const coachID = "11111111-2222-4333-8444-555555555555";
const viewedAt = "2026-11-11T14:00:00.123Z";

const mocks = vi.hoisted(() => ({ api: vi.fn(), identity: vi.fn() }));
vi.mock("@/components/auth-provider", () => ({ useAuth: () => ({ token: "main-admin" }) }));
vi.mock("@/components/coaching-gate", () => ({ CoachingGate: ({ children }: { children: ReactNode }) => children }));
vi.mock("@/components/confirm-dialog", () => ({ useConfirm: () => vi.fn() }));
vi.mock("@/components/identity-session", () => ({
  IdentitySession: ({ children }: { children: (token: string) => ReactNode }) => children("vault-admin"),
  identityFetch: (...args: unknown[]) => mocks.identity(...args),
}));
vi.mock("@/lib/api", async original => ({ ...await original<typeof import("@/lib/api")>(), apiFetch: (...args: unknown[]) => mocks.api(...args) }));

let container: HTMLDivElement;
let root: Root;
beforeEach(() => {
  (globalThis as typeof globalThis & { IS_REACT_ACT_ENVIRONMENT: boolean }).IS_REACT_ACT_ENVIRONMENT = true;
  mocks.api.mockReset(); mocks.identity.mockReset();
  container = document.createElement("div"); document.body.appendChild(container); root = createRoot(container);
});
afterEach(async () => { await act(async () => root.unmount()); container.remove(); });

async function type(input: Element, value: string) {
  await act(async () => {
    Object.getOwnPropertyDescriptor(HTMLInputElement.prototype, "value")!.set!.call(input, value);
    input.dispatchEvent(new Event("input", { bubbles: true }));
  });
}

async function view() {
  await act(async () => root.render(<Page />));
  const [id, purpose] = Array.from(container.querySelectorAll("input"));
  await type(id, coachID);
  await type(purpose, "開示請求の対応");
  const submit = Array.from(container.querySelectorAll("button")).find(b => b.textContent === "記録して見る")!;
  await act(async () => submit.click());
}

it("表示1回につき、識別子と表示時刻だけを本体 API に1回渡す", async () => {
  mocks.identity.mockResolvedValue({ identity, access_logs: [{ id: 9, viewer: "viewer-1", purpose: "開示請求の対応", created_at: viewedAt }] });
  mocks.api.mockResolvedValue(undefined);
  await view();
  expect(container.textContent).toContain(identity.legal_name);
  expect(mocks.api).toHaveBeenCalledTimes(1);
  const [path, init] = mocks.api.mock.calls[0] as [string, { method: string; token: string; body: string }];
  expect(path).toBe(`/v1/admin/coaching/coaches/${coachID}/identity-view-notice`);
  expect(init.method).toBe("POST");
  expect(init.token).toBe("main-admin");
  expect(JSON.parse(init.body)).toEqual({ viewed_at: viewedAt });
  for (const value of Object.values(identity)) {
    expect(JSON.stringify(mocks.api.mock.calls)).not.toContain(value);
  }
});

it("通知に失敗しても身元情報の表示は続き、エラーも出さない", async () => {
  mocks.identity.mockResolvedValue({ identity, access_logs: [{ id: 9, viewer: "viewer-1", purpose: "開示請求の対応", created_at: viewedAt }] });
  mocks.api.mockRejectedValue(new Error("mail_unavailable"));
  await view();
  expect(mocks.api).toHaveBeenCalledTimes(1);
  expect(container.textContent).toContain(identity.address);
  expect(container.querySelector('[role="alert"]')).toBeNull();
});

it("表示できなかったときは通知しない", async () => {
  mocks.identity.mockRejectedValue(new Error("対象の登録情報が見つかりません"));
  await view();
  expect(mocks.api).not.toHaveBeenCalled();
  expect(container.querySelector('[role="alert"]')?.textContent).toContain("見つかりません");
});
