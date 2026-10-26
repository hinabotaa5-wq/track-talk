/** @vitest-environment jsdom */
import { act, createElement, type ComponentProps } from "react";
import { createRoot, type Root } from "react-dom/client";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import { useCurrentMainTab } from "./bottom-nav";
import { HeaderAccountPanel } from "./header-account-panel";
import { FeedbackBox } from "./feedback-box";
import { NotificationBell } from "./notification-bell";
import { SenseNoteComposer } from "./sense-notes";

const apiFetch = vi.fn();
let pathname = "/";
const replace = vi.fn();
vi.mock("next/navigation", () => ({ usePathname: () => pathname, useRouter: () => ({ replace }) }));
const linkStatus = vi.hoisted(() => ({ pending: false }));
vi.mock("next/link", () => ({
  default: ({ href, children, className, onClick, ...props }: ComponentProps<"a"> & { href: string }) => (
    <a href={href} className={className} onClick={onClick} aria-label={props["aria-label"]} aria-expanded={props["aria-expanded"]}>{children}</a>
  ),
  useLinkStatus: () => linkStatus,
}));
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
  linkStatus.pending = false;
  sessionStorage.clear();
  localStorage.clear();
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

  it("マイページへの移動に時間がかかるあいだはクルクルを出す", async () => {
    vi.useFakeTimers();
    linkStatus.pending = true;
    await act(async () => root.render(<HeaderAccountPanel signedIn />));
    expect(container.querySelector('[role="status"]')).toBeNull();
    await act(async () => vi.advanceTimersByTime(200));
    expect(container.querySelector('[role="status"]')?.textContent).toBe("マイページを開いています");
    expect(container.querySelector("svg.animate-spin")).not.toBeNull();
  });

  it("マイページを開いている間はアイコンを選択状態にする", async () => {
    pathname = "/me";
    await act(async () => root.render(<HeaderAccountPanel signedIn />));
    expect(container.querySelector("a")?.className).toContain("bg-white/20");
  });

  it("ご意見を開いている間はアイコンを選択状態にする", async () => {
    await act(async () => root.render(<FeedbackBox />));
    const button = container.querySelector("button");
    expect(button?.className).not.toContain("bg-white/20");
    await click(button);
    expect(button?.className).toContain("bg-white/20");
  });

  it("マイページでも元の下部タブの選択を保持する", async () => {
    function Selection() { return <span>{useCurrentMainTab("/me")}</span>; }
    sessionStorage.setItem("account-origin", "/injury?tab=data");
    await act(async () => root.render(<Selection />));
    expect(container.textContent).toBe("injury");
  });

  it("ケガタブから開いた投稿の詳細でも選択色を保つ", async () => {
    localStorage.setItem("tracktalk:last-tab", "/injury?tab=data");
    function Selection() { return <span>{useCurrentMainTab("/posts/post-1")}</span>; }
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
    expect(bell?.className).toContain("bg-white/20");
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
