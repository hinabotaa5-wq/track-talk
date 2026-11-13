/** @vitest-environment jsdom */
import { act, type ReactNode } from "react";
import { createRoot, type Root } from "react-dom/client";
import { afterEach, beforeEach, expect, it, vi } from "vitest";

// 「コーチ」ボタンは、ログイン済みかつ GET /v1/coach/me の application が null でないときだけ出す（本体 #641）。
// status は問わない。読み込み中・取得失敗・未ログイン・応募なしでは出さない。
const apiFetch = vi.fn();
const auth = { token: "tok" as string | null };
vi.mock("next/link", () => ({
  default: ({ children, href, ...props }: { children: ReactNode; href: string }) => <a href={href} {...props}>{children}</a>,
}));
vi.mock("@/lib/api", async (original) => ({ ...await original<typeof import("@/lib/api")>(), apiFetch: (...args: unknown[]) => apiFetch(...args) }));
vi.mock("@/components/auth-provider", () => ({ useAuth: () => ({ token: auth.token }) }));
const { CoachButton } = await import("./coach-button");

let root: Root;
let container: HTMLDivElement;
beforeEach(() => {
  (globalThis as typeof globalThis & { IS_REACT_ACT_ENVIRONMENT: boolean }).IS_REACT_ACT_ENVIRONMENT = true;
  container = document.createElement("div"); document.body.appendChild(container); root = createRoot(container);
  apiFetch.mockReset(); auth.token = "tok";
});
afterEach(async () => { await act(async () => root.unmount()); container.remove(); });
const render = async () => { await act(async () => root.render(<CoachButton />)); await act(async () => { await Promise.resolve(); }); };
const button = () => container.querySelector('a[href="/coach"]');

it("未ログインでは出さず、応募状態も取得しない", async () => {
  auth.token = null;
  await render();
  expect(button()).toBeNull();
  expect(apiFetch).not.toHaveBeenCalled();
});

it("応募なし（application: null）では出さない", async () => {
  apiFetch.mockResolvedValue({ application: null });
  await render();
  expect(apiFetch).toHaveBeenCalledWith("/v1/coach/me", { token: "tok" });
  expect(button()).toBeNull();
});

it("承認済みの応募があるときは「コーチ」で /coach に出す", async () => {
  apiFetch.mockResolvedValue({ application: { status: "approved" } });
  await render();
  expect(button()?.textContent).toContain("コーチ");
});

it("承認前（pending）の応募でも出す（status は問わない）", async () => {
  apiFetch.mockResolvedValue({ application: { status: "pending" } });
  await render();
  expect(button()?.textContent).toContain("コーチ");
  expect(button()?.getAttribute("href")).toBe("/coach");
});

it("読み込み中は出さない", async () => {
  apiFetch.mockReturnValue(new Promise(() => {}));
  await render();
  expect(button()).toBeNull();
});

it("取得失敗では出さない", async () => {
  apiFetch.mockRejectedValue(new Error("offline"));
  await render();
  expect(button()).toBeNull();
});
