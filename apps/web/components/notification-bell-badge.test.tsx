/** @vitest-environment jsdom */
import { act, type ComponentProps } from "react";
import { createRoot, type Root } from "react-dom/client";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import { NotificationBell } from "./notification-bell";

const badge = vi.hoisted(() => ({ applyAppBadge: vi.fn(), clearAppBadge: vi.fn() }));
vi.mock("@/lib/app-badge", () => ({
  applyAppBadge: (...args: unknown[]) => badge.applyAppBadge(...args),
  clearAppBadge: (...args: unknown[]) => badge.clearAppBadge(...args),
}));

const auth = vi.hoisted(() => ({ token: "test-token" as string | null }));
vi.mock("./auth-provider", () => ({ useAuth: () => ({ token: auth.token }) }));
vi.mock("next/navigation", () => ({ usePathname: () => "/", useRouter: () => ({ replace: () => {} }) }));
vi.mock("next/link", () => ({
  default: ({ href, children, onClick }: ComponentProps<"a"> & { href: string }) => (
    <a href={href} onClick={onClick}>{children}</a>
  ),
}));
vi.mock("next/dynamic", () => ({ default: () => () => null }));

const apiFetch = vi.fn();
vi.mock("@/lib/api", () => ({
  apiFetch: (...args: unknown[]) => apiFetch(...args),
  apiErrorText: (_err: unknown, fallback: string) => fallback,
}));

let serverUnread = 3;
let serverItems = [
  { id: "a", kind: "message", title: "お知らせA", read: false, url: "/memo", created_at: "2026-10-23T00:00:00Z" },
  { id: "b", kind: "message", title: "お知らせB", read: false, url: "/memo", created_at: "2026-10-22T00:00:00Z" },
  { id: "c", kind: "message", title: "お知らせC", read: false, url: "/memo", created_at: "2026-10-21T00:00:00Z" },
];

let root: Root;
let container: HTMLDivElement;
beforeEach(() => {
  (globalThis as typeof globalThis & { IS_REACT_ACT_ENVIRONMENT: boolean }).IS_REACT_ACT_ENVIRONMENT = true;
  apiFetch.mockReset();
  badge.applyAppBadge.mockReset();
  badge.clearAppBadge.mockReset();
  auth.token = "test-token";
  serverUnread = 3;
  serverItems = [
    { id: "a", kind: "message", title: "お知らせA", read: false, url: "/memo", created_at: "2026-10-23T00:00:00Z" },
    { id: "b", kind: "message", title: "お知らせB", read: false, url: "/memo", created_at: "2026-10-22T00:00:00Z" },
    { id: "c", kind: "message", title: "お知らせC", read: false, url: "/memo", created_at: "2026-10-21T00:00:00Z" },
  ];
  apiFetch.mockImplementation(async (path: string) => {
    if (path === "/v1/me/notifications/read") {
      serverUnread = 0;
      serverItems = serverItems.map((item) => ({ ...item, read: true }));
      return {};
    }
    if (path.startsWith("/v1/me/notifications/") && path.endsWith("/read")) {
      const id = path.split("/")[4];
      serverItems = serverItems.map((item) => (item.id === id ? { ...item, read: true } : item));
      serverUnread = serverItems.filter((item) => !item.read).length;
      return {};
    }
    if (path.startsWith("/v1/me/notifications")) {
      return { unread_count: serverUnread, notifications: serverItems };
    }
    return {};
  });
  container = document.createElement("div");
  document.body.appendChild(container);
  root = createRoot(container);
});
afterEach(async () => {
  await act(async () => root.unmount());
  container.remove();
});

async function click(el: Element | null) {
  if (!el) throw new Error("クリック対象がありません");
  await act(async () => (el as HTMLElement).click());
}

describe("通知ベルのアプリアイコンバッジ", () => {
  it("起動時の取得数をバッジへ渡す", async () => {
    await act(async () => root.render(<NotificationBell />));
    expect(badge.applyAppBadge).toHaveBeenCalledWith(3);
  });

  it("フォアグラウンド復帰で最新数を取り直して渡す", async () => {
    await act(async () => root.render(<NotificationBell />));
    badge.applyAppBadge.mockClear();
    serverUnread = 5;
    Object.defineProperty(document, "visibilityState", { configurable: true, value: "visible" });
    document.dispatchEvent(new Event("visibilitychange"));
    await act(async () => {});
    expect(badge.applyAppBadge).toHaveBeenLastCalledWith(5);
  });

  it("すべて既読の完了後に0を渡し、ベルと一致する", async () => {
    await act(async () => root.render(<NotificationBell />));
    await click(container.querySelector("button"));
    await click([...document.querySelectorAll("button")].find((button) => button.textContent === "すべて既読") ?? null);
    await act(async () => {});
    expect(badge.applyAppBadge).toHaveBeenLastCalledWith(0);
    expect(container.querySelector("button")?.getAttribute("aria-label")).toBe("通知");
  });

  it("既読化の完了後に最新数を渡す", async () => {
    await act(async () => root.render(<NotificationBell />));
    await click(container.querySelector("button"));
    await act(async () => {});
    await click(document.querySelector('li a'));
    await act(async () => {});
    expect(badge.applyAppBadge).toHaveBeenLastCalledWith(2);
    expect(container.querySelector("button")?.getAttribute("aria-label")).toContain("未読2件");
  });

  it("未ログインでは付けず、ログアウトで消す", async () => {
    auth.token = null;
    await act(async () => root.render(<NotificationBell />));
    expect(apiFetch).not.toHaveBeenCalled();
    expect(badge.applyAppBadge).not.toHaveBeenCalled();
    expect(badge.clearAppBadge).toHaveBeenCalledTimes(1);
  });
});
