/** @vitest-environment jsdom */
import { act } from "react";
import { createRoot, type Root } from "react-dom/client";
import { afterEach, beforeEach, expect, it, vi } from "vitest";
import { ReferralInput } from "./coaching-referral";

const mocks = vi.hoisted(() => ({ api: vi.fn() }));
vi.mock("@/lib/api", async (original) => ({ ...await original<typeof import("@/lib/api")>(), apiFetch: (...args: unknown[]) => mocks.api(...args) }));

let container: HTMLDivElement;
let root: Root;
beforeEach(() => {
  (globalThis as typeof globalThis & { IS_REACT_ACT_ENVIRONMENT: boolean }).IS_REACT_ACT_ENVIRONMENT = true;
  mocks.api.mockReset();
  container = document.createElement("div"); document.body.appendChild(container); root = createRoot(container);
});
afterEach(async () => { await act(async () => root.unmount()); container.remove(); });

const note = "紹介IDを使って購入すると、この購入ではあなたの紹介IDは発行されません。";

async function renderInput(code: string, referral: unknown, warning = false) {
  mocks.api.mockResolvedValue({ referral });
  await act(async () => root.render(<ReferralInput token="test" code={code} onChange={() => {}} warning={warning} />));
}

it.each([
  { name: "IDを持たない人", referral: null },
  { name: "周が終了した人", referral: { code: "K7M3QX", active: false, friend_count: 2, granted_at: "2026-11-14T09:00:00Z", expires_at: null } },
])("有効な紹介IDを持たない人が紹介IDを入れると、自分のIDが発行されない旨を添える（$name）", async ({ referral }) => {
  await renderInput("ABCDEF", referral);
  expect(container.textContent).toContain(note);
});

it.each([
  { name: "空欄", code: "", referral: null, warning: false },
  { name: "有効なIDを持つ人", code: "ABCDEF", referral: { code: "K7M3QX", active: true, friend_count: 0, granted_at: null, expires_at: null }, warning: false },
  { name: "使えないIDの警告中", code: "ABCDEF", referral: null, warning: true },
])("注記を出さない（$name）", async ({ code, referral, warning }) => {
  await renderInput(code, referral, warning);
  expect(container.textContent).not.toContain(note);
});
