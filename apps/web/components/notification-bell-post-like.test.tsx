/** @vitest-environment jsdom */
import { act, type ComponentProps } from "react";
import { createRoot, type Root } from "react-dom/client";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import { NotificationBell } from "./notification-bell";

// 投稿へのいいねの通知(post_like)をベルに出す(本体 #608)。

vi.mock("@/lib/app-badge", () => ({ applyAppBadge: () => {}, clearAppBadge: () => {} }));
vi.mock("./auth-provider", () => ({ useAuth: () => ({ token: "test-token" }) }));
vi.mock("next/navigation", () => ({ usePathname: () => "/", useRouter: () => ({ replace: () => {} }) }));
vi.mock("next/link", () => ({
  default: ({ href, children, onClick }: ComponentProps<"a"> & { href: string }) => (
    <a href={href} onClick={onClick}>{children}</a>
  ),
}));
vi.mock("next/dynamic", () => ({ default: () => () => null }));

const locale = vi.hoisted(() => ({ value: "ja" as "ja" | "en" }));
vi.mock("@/components/locale-provider", () => ({ useLocale: () => locale.value }));

vi.mock("@/lib/api", () => ({
  apiFetch: async (path: string) => {
    if (path.startsWith("/v1/me/notifications") && !path.endsWith("/read")) {
      return {
        unread_count: 1,
        notifications: [
          { id: "n1", kind: "post_like", post_id: "p-123", post_title: "練習日誌", read: false, created_at: "2026-11-13T00:00:00Z" },
        ],
      };
    }
    return {};
  },
  apiErrorText: (_err: unknown, fallback: string) => fallback,
}));

let root: Root;
let container: HTMLDivElement;
beforeEach(() => {
  (globalThis as typeof globalThis & { IS_REACT_ACT_ENVIRONMENT: boolean }).IS_REACT_ACT_ENVIRONMENT = true;
  container = document.createElement("div");
  document.body.appendChild(container);
  root = createRoot(container);
});
afterEach(async () => {
  await act(async () => root.unmount());
  container.remove();
});

async function openBell() {
  await act(async () => root.render(<NotificationBell />));
  await act(async () => (container.querySelector("button") as HTMLButtonElement).click());
  await act(async () => {});
  return document.querySelector("li a") as HTMLAnchorElement | null;
}

describe("いいねの通知", () => {
  it("日本語で「いいねがありました」と出し、その投稿へ行く", async () => {
    locale.value = "ja";
    const link = await openBell();
    expect(link?.textContent).toContain("いいねがありました");
    expect(link?.getAttribute("href")).toBe("/posts/p-123");
  });

  it("英語表示では英語の文になる", async () => {
    locale.value = "en";
    const link = await openBell();
    expect(link?.textContent).toContain("New like");
    expect(link?.textContent).not.toContain("いいね");
    expect(link?.getAttribute("href")).toBe("/posts/p-123");
  });
});
