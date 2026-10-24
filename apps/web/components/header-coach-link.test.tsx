/** @vitest-environment jsdom */
import { act, type ReactNode } from "react";
import { createRoot, type Root } from "react-dom/client";
import { afterEach, beforeEach, expect, it, vi } from "vitest";
import { HeaderCoachLink } from "./header-coach-link";
let token: string | null = "test";
const apiFetch = vi.fn();
vi.mock("./auth-provider", () => ({ useAuth: () => ({ token }) }));
vi.mock("next/navigation", () => ({ usePathname: () => "/" }));
vi.mock("next/link", () => ({ default: ({ href, children, ...props }: { href: string; children: ReactNode }) => <a href={href} {...props}>{children}</a> }));
vi.mock("@/lib/api", () => ({ apiFetch: (...args: unknown[]) => apiFetch(...args) }));
vi.mock("./leave-guard", () => ({ useGuardedLinkClick: () => vi.fn() }));
let root: Root;
let container: HTMLDivElement;
beforeEach(() => {
  (globalThis as typeof globalThis & { IS_REACT_ACT_ENVIRONMENT: boolean }).IS_REACT_ACT_ENVIRONMENT = true;
  token = "test"; apiFetch.mockReset();
  container = document.createElement("div"); document.body.appendChild(container); root = createRoot(container);
});
afterEach(async () => { await act(async () => root.unmount()); container.remove(); });
it.each([null, "pending", "rejected", "suspended", "approved"])("状態 %s：承認済みコーチだけに専用画面のリンクを出す", async (status) => {
  apiFetch.mockResolvedValue({ application: status ? { status } : null });
  await act(async () => root.render(<HeaderCoachLink />));
  expect(container.querySelector('a[aria-label="コーチ専用画面"]')?.getAttribute("href") ?? null).toBe(status === "approved" ? "/coach" : null);
});
it("承認後に画面へ戻ると更新し、ログアウトで消す", async () => {
  apiFetch.mockResolvedValue({ application: { status: "pending" } });
  await act(async () => root.render(<HeaderCoachLink />));
  expect(container.querySelector("a")).toBeNull();
  apiFetch.mockResolvedValue({ application: { status: "approved" } });
  await act(async () => window.dispatchEvent(new Event("focus")));
  expect(container.querySelector("a")).not.toBeNull();
  token = null;
  await act(async () => root.render(<HeaderCoachLink />));
  expect(container.querySelector("a")).toBeNull();
});
it("別アカウントには前のコーチアイコンを表示しない", async () => {
  apiFetch.mockResolvedValue({ application: { status: "approved" } });
  await act(async () => root.render(<HeaderCoachLink />));
  token = "another-user";
  apiFetch.mockReturnValue(new Promise(() => {}));
  await act(async () => root.render(<HeaderCoachLink />));
  expect(container.querySelector("a")).toBeNull();
});
