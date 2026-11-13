/** @vitest-environment jsdom */
import { act, createElement, type ComponentProps } from "react";
import { createRoot, type Root } from "react-dom/client";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import { useCurrentMainTab } from "./bottom-nav";
import { Header } from "./header";
import { HeaderAccountPanel } from "./header-account-panel";
import { AppLogo } from "./app-icon-mark";
import { FeedbackBox } from "./feedback-box";
import { InstagramLink } from "./instagram-link";
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
vi.mock("./auth-provider", () => ({ useAuth: () => ({ token: "test-token", session: { user: { id: "user-1" } }, profile: null }) }));
vi.mock("./leave-guard", () => ({
  useLeaveGuard: () => ({ confirmLeave: () => false }),
  useGuardedLinkClick: () => () => {},
}));
vi.mock("./today-meet-balloon", () => ({ TodayMeetBalloon: () => null }));
vi.mock("./menu-contest-bar", () => ({ MenuContestBar: () => null }));
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

  it("マイページへの移動に時間がかかっても、アイコンは回さずに薄い色にする", async () => {
    vi.useFakeTimers();
    linkStatus.pending = true;
    await act(async () => root.render(<HeaderAccountPanel signedIn />));
    expect(container.querySelector("[aria-busy]")).toBeNull();
    await act(async () => vi.advanceTimersByTime(200));
    expect(container.querySelector("[aria-busy]")).not.toBeNull();
    expect(container.querySelector("svg.animate-spin")).toBeNull();
  });

  it("マイページを開いている間はアイコンを選択状態にする", async () => {
    pathname = "/me";
    await act(async () => root.render(<HeaderAccountPanel signedIn />));
    expect(container.querySelector("a")?.className).toContain("bg-white/20");
  });

  it("Instagramは別タブで開く外部リンクで、ご意見と同じ大きさの領域にする", async () => {
    await act(async () => root.render(<InstagramLink />));
    const link = container.querySelector("a")!;
    expect(link.getAttribute("href")).toBe("https://www.instagram.com/tracktalk.office/");
    expect(link.getAttribute("target")).toBe("_blank");
    expect(link.getAttribute("rel")).toContain("noopener");
    expect(link.getAttribute("aria-label")).toContain("@tracktalk.office");
    expect(link.className).toContain("size-11");
    const icon = link.querySelector("svg");
    expect(icon?.getAttribute("aria-hidden")).toBe("true");
    expect(icon?.getAttribute("class")).toContain("size-5");
  });

  it("Instagramはご意見の直左に出る", async () => {
    apiFetch.mockResolvedValue({ unread_count: 0, notifications: [] });
    await act(async () => root.render(<Header />));
    const header = container.querySelector("header")!;
    const controls = [...header.querySelectorAll("a, button")];
    const instagramIndex = controls.findIndex((el) => el.getAttribute("aria-label")?.includes("@tracktalk.office"));
    const feedbackIndex = controls.findIndex((el) => el.getAttribute("aria-label") === "ご意見");
    expect(instagramIndex).toBeGreaterThanOrEqual(0);
    expect(feedbackIndex).toBe(instagramIndex + 1);
  });

  it("ロゴは24pxのマークと16pxの文字にする（文言とリンク先は不変）", async () => {
    await act(async () => root.render(<AppLogo />));
    expect(container.textContent).toContain("TrackTalk");
    expect(container.querySelector("svg")?.getAttribute("class")).toContain("size-6");
    const text = [...container.querySelectorAll("span")].find((el) => el.children.length === 0 && el.textContent === "TrackTalk")!;
    expect(text.getAttribute("class")).toContain("text-base");
  });

  it("ご意見を開いている間はアイコンを選択状態にする", async () => {
    await act(async () => root.render(<FeedbackBox />));
    const button = container.querySelector("button");
    expect(button?.className).not.toContain("bg-white/20");
    await click(button);
    expect(button?.className).toContain("bg-white/20");
  });

  it.each([
    ["/memo", "memo"],
    ["/injury?tab=data", "injury"],
    ["/competitions", "competitions"],
    ["/coaching", "coaching"],
    ["/", "home"],
  ])("マイページでも %s の下部タブの選択を保持する", async (origin, tab) => {
    function Selection() { return <span>{useCurrentMainTab("/me")}</span>; }
    sessionStorage.setItem("account-origin", origin);
    await act(async () => root.render(<Selection />));
    expect(container.textContent).toBe(tab);
  });

  it("ケガタブから開いた投稿の詳細でも選択色を保つ", async () => {
    localStorage.setItem("tracktalk:last-tab", "/injury?tab=data");
    function Selection() { return <span>{useCurrentMainTab("/posts/post-1")}</span>; }
    await act(async () => root.render(<Selection />));
    expect(container.textContent).toBe("injury");
  });

  it("未読と既読を表示し、すべて既読で状態と件数が変わる", async () => {
    // 既読化のあとは正本を取り直すので、サーバー側も既読化を反映する。
    let readAll = false;
    apiFetch.mockImplementation(async (path: string, options?: { method?: string }) => {
      if (options?.method === "POST") {
        readAll = true;
        return {};
      }
      return { unread_count: readAll ? 0 : 1, notifications: [
        { id: "unread", kind: "message", title: "新しいお知らせ", read: readAll, url: "/memo", created_at: "2026-10-23T00:00:00Z" },
        { id: "read", kind: "message", title: "前のお知らせ", read: true, url: "/memo", created_at: "2026-10-22T00:00:00Z" },
      ] };
    });
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
  await act(async () => root.render(createElement(SenseNoteComposer, { token: "test-token", writtenToday: false, streak: 0, onCreated })));
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
  expect(container.querySelector('[role="status"]')?.textContent).toBe("また明日も書こう！");
});

it("保存済みの画面でも連続日数の一文だけを表示する", async () => {
  await act(async () => root.render(<SenseNoteComposer token="test-token" writtenToday streak={6} onCreated={vi.fn()} />));
  expect(container.textContent).toBe("6日続けて書けてる。また明日も書こう！");
});
