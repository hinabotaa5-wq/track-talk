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

const auth = vi.hoisted(() => ({ token: "test-token" as string | null, userId: "user-1" as string | null }));
vi.mock("./auth-provider", () => ({ useAuth: () => ({ token: auth.token, session: auth.userId ? { user: { id: auth.userId } } : null }) }));
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
  auth.userId = "user-1";
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

// ---- 本体 #1009：表示中の利用者の分だけを出す ----
type Feed = { unread_count: number; notifications: typeof serverItems };
function deferredFeed() {
  let resolve!: (value: Feed) => void;
  const promise = new Promise<Feed>((res) => { resolve = res; });
  return { promise, resolve };
}
const feedB = (): Feed => ({
  unread_count: 1,
  notifications: [{ id: "z", kind: "message", title: "Bさんのお知らせ", read: false, url: "/memo", created_at: "2026-10-24T00:00:00Z" }],
});
const bell = () => container.querySelector("button")!;
const switchUser = async (token: string | null, userId: string | null) => {
  auth.token = token;
  auth.userId = userId;
  await act(async () => root.render(<NotificationBell />));
};
const firePageshow = async (persisted: boolean) => {
  const event = new Event("pageshow");
  Object.defineProperty(event, "persisted", { value: persisted });
  await act(async () => { window.dispatchEvent(event); });
  await act(async () => {});
};
// token ごとに応答を分ける。B の応答は試験側で返すまで保留する。
function routeByToken(pendingB: Promise<Feed>[]) {
  apiFetch.mockImplementation(async (path: string, init?: { token?: string }) => {
    if (!path.startsWith("/v1/me/notifications") || path.endsWith("/read")) return {};
    if (init?.token === "token-b") return pendingB.shift() ?? feedB();
    return { unread_count: serverUnread, notifications: serverItems };
  });
}

describe("利用者の切替と履歴復帰（本体 #1009）", () => {
  it("T9：A の未読件数・一覧のあと B に切り替えると、B の取得完了まで A の件数・一覧を出さず、パネルは閉じる。完了後は B の分だけを出す", async () => {
    const b1 = deferredFeed();
    routeByToken([b1.promise]);
    await act(async () => root.render(<NotificationBell />));
    await click(bell());
    await act(async () => {});
    expect(bell().getAttribute("aria-label")).toContain("未読3件");
    expect(bell().getAttribute("aria-expanded")).toBe("true");
    expect(document.body.textContent).toContain("お知らせA");

    await switchUser("token-b", "user-b");
    expect(bell().getAttribute("aria-label")).toBe("通知");
    expect(bell().textContent).toBe("");
    expect(bell().getAttribute("aria-expanded")).toBe("false");
    expect(document.body.textContent).not.toContain("お知らせA");

    await act(async () => { b1.resolve(feedB()); });
    expect(bell().getAttribute("aria-label")).toContain("未読1件");
    expect(bell().getAttribute("aria-expanded")).toBe("false");

    await click(bell());
    await act(async () => {});
    expect(document.body.textContent).toContain("Bさんのお知らせ");
    expect(document.body.textContent).not.toContain("お知らせA");
  });

  it("T9：パネルを閉じたまま切り替えても、B の一覧を開いたときに A の一覧を出さない", async () => {
    const b1 = deferredFeed();
    const b2 = deferredFeed();
    routeByToken([b1.promise, b2.promise]);
    await act(async () => root.render(<NotificationBell />));
    await click(bell());
    await act(async () => {});
    await click(bell());
    await switchUser("token-b", "user-b");
    await act(async () => { b1.resolve(feedB()); });
    await click(bell());
    expect(document.body.textContent).not.toContain("お知らせA");
    await act(async () => { b2.resolve(feedB()); });
    expect(document.body.textContent).toContain("Bさんのお知らせ");
  });

  it("T10：pageshow（persisted）で現在の利用者の分を取り直し、persisted でなければ取り直さない", async () => {
    await act(async () => root.render(<NotificationBell />));
    apiFetch.mockClear();
    await firePageshow(false);
    expect(apiFetch).not.toHaveBeenCalled();
    serverUnread = 7;
    await firePageshow(true);
    expect(apiFetch).toHaveBeenCalledTimes(1);
    expect(apiFetch.mock.calls[0][1]).toEqual({ token: "test-token" });
    expect(bell().getAttribute("aria-label")).toContain("未読7件");
    expect(badge.applyAppBadge).toHaveBeenLastCalledWith(7);
  });

  it("T11：切替時にアプリバッジの前の利用者の件数を消し、遅れて返った A の件数を再適用しない", async () => {
    let resolveA!: (value: Feed) => void;
    const b1 = deferredFeed();
    apiFetch.mockImplementationOnce(async () => ({ unread_count: 3, notifications: serverItems }));
    await act(async () => root.render(<NotificationBell />));
    expect(badge.applyAppBadge).toHaveBeenLastCalledWith(3);

    // A の取り直しが返る前に B へ切り替える。
    apiFetch.mockImplementationOnce(() => new Promise<Feed>((res) => { resolveA = res; }));
    Object.defineProperty(document, "visibilityState", { configurable: true, value: "visible" });
    await act(async () => { document.dispatchEvent(new Event("visibilitychange")); });
    routeByToken([b1.promise]);
    badge.applyAppBadge.mockClear();
    badge.clearAppBadge.mockClear();
    await switchUser("token-b", "user-b");
    expect(badge.clearAppBadge).toHaveBeenCalled();

    await act(async () => { resolveA({ unread_count: 9, notifications: serverItems }); });
    expect(badge.applyAppBadge).not.toHaveBeenCalled();
    expect(bell().getAttribute("aria-label")).toBe("通知");

    await act(async () => { b1.resolve(feedB()); });
    expect(badge.applyAppBadge).toHaveBeenLastCalledWith(1);
  });

  it("T12：同じ利用者のまま token 文字列だけが変わったら、件数を消さずに取り直し、完了後に新しい値へ更新する", async () => {
    await act(async () => root.render(<NotificationBell />));
    expect(bell().getAttribute("aria-label")).toContain("未読3件");
    let resolveNext!: (value: Feed) => void;
    apiFetch.mockImplementationOnce(() => new Promise<Feed>((res) => { resolveNext = res; }));
    badge.clearAppBadge.mockClear();
    await switchUser("test-token-2", "user-1");
    expect(bell().getAttribute("aria-label")).toContain("未読3件");
    expect(badge.clearAppBadge).not.toHaveBeenCalled();
    expect(apiFetch.mock.lastCall?.[1]).toEqual({ token: "test-token-2" });
    await act(async () => { resolveNext({ unread_count: 4, notifications: serverItems }); });
    expect(bell().getAttribute("aria-label")).toContain("未読4件");
  });
});
