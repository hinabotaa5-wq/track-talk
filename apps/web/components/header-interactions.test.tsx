/** @vitest-environment jsdom */
import { act, createElement, type ReactNode } from "react";
import { createRoot, type Root } from "react-dom/client";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import { useCurrentMainTab } from "./bottom-nav";
import { HeaderAccountPanel } from "./header-account-panel";
import { NotificationBell } from "./notification-bell";
import { SenseNoteComposer } from "./sense-notes";

const apiFetch = vi.fn();
let pathname = "/";
const replace = vi.fn();
vi.mock("next/navigation", () => ({ usePathname: () => pathname, useRouter: () => ({ replace }) }));
vi.mock("next/link", () => ({ default: ({ href, children, ...props }: { href: string; children: ReactNode }) => <a href={href} {...props}>{children}</a> }));
vi.mock("next/dynamic", () => ({ default: () => () => <p>メニューの内容</p> }));
vi.mock("./auth-provider", () => ({ useAuth: () => ({ token: "test-token" }) }));
vi.mock("@/lib/api", () => ({
  apiFetch: (...args: unknown[]) => apiFetch(...args),
  apiErrorText: (_err: unknown, fallback: string) => fallback,
}));

let root: Root;
let container: HTMLDivElement;
beforeEach(() => {
  (globalThis as typeof globalThis & { IS_REACT_ACT_ENVIRONMENT: boolean }).IS_REACT_ACT_ENVIRONMENT = true;
  apiFetch.mockReset();
  pathname = "/";
  replace.mockReset();
  sessionStorage.clear();
  container = document.createElement("div");
  document.body.appendChild(container);
  root = createRoot(container);
});
afterEach(async () => {
  await act(async () => root.unmount());
  container.remove();
  vi.useRealTimers();
});

async function click(el: Element | null) {
  if (!el) throw new Error("クリック対象がありません");
  await act(async () => (el as HTMLElement).click());
}

describe("ヘッダーの開閉", () => {
  for (const admin of [true]) {
    it(`${admin ? "管理" : "マイページ"}を同じアイコンで開閉し、Escapeでも閉じる`, async () => {
      await act(async () => root.render(<HeaderAccountPanel admin={admin} signedIn />));
      const button = container.querySelector("button");
      expect(button?.getAttribute("aria-expanded")).toBe("false");
      await click(button);
      expect(button?.getAttribute("aria-expanded")).toBe("true");
      expect(document.querySelector('[role="dialog"]')?.textContent).toContain("メニューの内容");
      await click(button);
      expect(document.querySelector('[role="dialog"]')).toBeNull();
      await click(button);
      await act(async () => document.dispatchEvent(new KeyboardEvent("keydown", { key: "Escape" })));
      expect(button?.getAttribute("aria-expanded")).toBe("false");
    });
  }

  it("マイページは独立したページへ移動する", async () => {
    await act(async () => root.render(<HeaderAccountPanel signedIn />));
    expect(container.querySelector('a[aria-label="マイページ"]')?.getAttribute("href")).toBe("/me");
    expect(container.querySelector("button")).toBeNull();
    expect(document.querySelector('[role="dialog"]')).toBeNull();
    await act(async () => root.render(<HeaderAccountPanel signedIn={false} />));
    expect(container.querySelector("a")?.getAttribute("href")).toBe("/login");
  });

  it("マイページのアイコンをもう一度押すと元のタブと検索条件に戻る", async () => {
    sessionStorage.setItem("account-origin", "/injury?tab=data");
    pathname = "/me";
    await act(async () => root.render(<HeaderAccountPanel signedIn />));
    const link = container.querySelector("a")!;
    expect(link.getAttribute("aria-expanded")).toBe("true");
    await click(link);
    expect(replace).toHaveBeenCalledWith("/injury?tab=data");
  });

  it("マイページでも元の下部タブの選択を保持する", async () => {
    function Selection() { return <span>{useCurrentMainTab("/me")}</span>; }
    sessionStorage.setItem("account-origin", "/injury?tab=data");
    await act(async () => root.render(<Selection />));
    expect(container.textContent).toBe("injury");
  });

  it("未読と既読を表示し、すべて既読で状態と件数が変わる", async () => {
    apiFetch.mockResolvedValue({ unread_count: 1, notifications: [
      { id: "unread", kind: "message", title: "新しいお知らせ", read: false, url: "/memo", created_at: "2026-10-23T00:00:00Z" },
      { id: "read", kind: "message", title: "前のお知らせ", read: true, url: "/memo", created_at: "2026-10-22T00:00:00Z" },
    ] });
    await act(async () => root.render(<NotificationBell />));
    const bell = container.querySelector("button");
    expect(bell?.getAttribute("aria-label")).toContain("未読1件");
    await click(bell);
    const rows = [...document.querySelectorAll("li")];
    expect(rows[0].textContent).toContain("未読");
    expect(rows[1].textContent).toContain("既読");
    expect(rows[0].className).not.toBe(rows[1].className);
    const markAll = [...document.querySelectorAll("button")].find((button) => button.textContent === "すべて既読");
    await click(markAll ?? null);
    expect(apiFetch).toHaveBeenCalledWith("/v1/me/notifications/read", { method: "POST", token: "test-token" });
    expect(rows.every((row) => row.textContent?.includes("既読"))).toBe(true);
    expect(bell?.getAttribute("aria-label")).toBe("通知");
  });
});

it("感覚メモの日付は日本時間の今日に固定し、保存日はAPIに任せる", async () => {
  vi.useFakeTimers();
  vi.setSystemTime(new Date("2026-10-22T16:00:00Z")); // 日本時間は10月23日
  const onCreated = vi.fn();
  apiFetch.mockResolvedValue({ note: { id: "new" }, stats: { total: 1, days: 1, streak: 1, today: 1 } });
  await act(async () => root.render(createElement(SenseNoteComposer, { token: "test-token", writtenToday: false, onCreated })));
  expect(container.querySelector("time")?.dateTime).toBe("2026-10-23");
  expect(container.querySelector("input")).toBeNull();
  expect(container.textContent).not.toContain("日本時間の今日の日付で保存します");
  expect(container.querySelector("select")).toBeNull();
  const textarea = container.querySelector("textarea")!;
  expect(textarea.placeholder).toBe("例) リズムをつけて走ったらリラックスできた");
  await act(async () => {
    Object.getOwnPropertyDescriptor(HTMLTextAreaElement.prototype, "value")!.set!.call(textarea, "リズムよく走れた");
    textarea.dispatchEvent(new Event("input", { bubbles: true }));
  });
  await act(async () => container.querySelector("form")!.dispatchEvent(new Event("submit", { bubbles: true, cancelable: true })));
  expect(apiFetch).toHaveBeenCalledWith("/v1/me/sense-notes", expect.objectContaining({ body: JSON.stringify({ body: "リズムよく走れた" }) }));
  expect(onCreated).toHaveBeenCalledOnce();
});
