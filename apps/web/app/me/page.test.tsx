/** @vitest-environment jsdom */
import { act } from "react";
import { createRoot, type Root } from "react-dom/client";
import { afterEach, beforeEach, expect, it, vi } from "vitest";
import { AuthProvider } from "@/components/auth-provider";
import { ConfirmProvider } from "@/components/confirm-dialog";
import MePage from "./page";
const mock = vi.hoisted(() => ({
  signOut: vi.fn(), getSession: vi.fn(), unsubscribe: vi.fn(), replace: vi.fn(), refresh: vi.fn(),
  apiFetch: vi.fn(), referral: vi.fn(),
  onAuthStateChange: vi.fn(),
}));
vi.mock("@/lib/supabase", () => ({ createClient: () => ({ auth: {
  signOut: mock.signOut, getSession: mock.getSession, onAuthStateChange: mock.onAuthStateChange,
} }) }));
vi.mock("@/lib/api", () => ({
  apiFetch: (path: string, ...args: unknown[]) => path === "/v1/me/coaching/referral"
    ? mock.referral()
    : mock.apiFetch(path, ...args),
}));
const flags = { coaching: false };
vi.mock("@/lib/features", async (original) => ({
  ...await original<typeof import("@/lib/features")>(),
  get coachingEnabled() { return flags.coaching; },
}));
vi.mock("@/lib/sign-out", () => ({
  signOutOnThisDevice: mock.unsubscribe,
  clearPersonalStorage: vi.fn(),
}));
vi.mock("next/navigation", () => ({ useRouter: () => ({ replace: mock.replace, refresh: mock.refresh }) }));
let root: Root, container: HTMLDivElement;
beforeEach(() => {
  (globalThis as { IS_REACT_ACT_ENVIRONMENT?: boolean }).IS_REACT_ACT_ENVIRONMENT = true;
  flags.coaching = false;
  vi.clearAllMocks();
  vi.stubEnv("NEXT_PUBLIC_SUPABASE_URL", "https://example.supabase.co");
  vi.stubEnv("NEXT_PUBLIC_SUPABASE_ANON_KEY", "test-key");
  mock.apiFetch.mockResolvedValue({ id: "user-1", is_admin: false });
  mock.referral.mockResolvedValue({ referral: null });
  mock.onAuthStateChange.mockReturnValue({ data: { subscription: { unsubscribe: vi.fn() } } });
  mock.signOut.mockResolvedValue({ error: null });
  mock.unsubscribe.mockImplementation(async (_userId, signOut) => signOut());
  mock.getSession.mockResolvedValue({ data: { session: null } });
  container = document.createElement("div"); document.body.appendChild(container); root = createRoot(container);
});
afterEach(async () => {
  await act(async () => root.unmount());
  container.remove();
  vi.unstubAllEnvs();
});
async function showPage(page = <MePage />) {
  await act(async () => root.render(<AuthProvider><ConfirmProvider>{page}</ConfirmProvider></AuthProvider>));
}
async function render() {
  await showPage();
  await vi.waitFor(() => expect(mock.onAuthStateChange).toHaveBeenCalledOnce());
}
async function signIn(event = "INITIAL_SESSION", id = "user-1") {
  await act(async () => mock.onAuthStateChange.mock.calls[0][0](event, { access_token: "token", user: { id } }));
}
async function renderSignedIn() {
  await render();
  await signIn();
}
async function logout() {
  await act(async () => [...container.querySelectorAll("button")].find((b) => b.textContent === "ログアウト")!.click());
  await act(async () => container.querySelector<HTMLButtonElement>('[role="alertdialog"] button')!.click());
}
const adminLink = () => container.querySelector('a[href="/admin"]');
const profileStatus = () => container.querySelector('nav [role="status"][aria-busy="true"]');

it.each([true, false])("プロフィールの応答が遅い間は確認中と表示し、完了後は管理者だけに入口を表示する (is_admin=%s)", async (isAdmin) => {
  let finish!: (profile: unknown) => void;
  mock.apiFetch.mockReturnValue(new Promise((resolve) => { finish = resolve; }));
  await renderSignedIn();
  expect(mock.apiFetch).toHaveBeenCalledExactlyOnceWith("/v1/me", { token: "token", signal: expect.any(AbortSignal) });
  expect(container.textContent).toContain("自分の投稿");
  expect(container.querySelector('a[href="/me/install"]')?.textContent).toContain("ホーム画面に追加して使う");
  expect(adminLink()).toBeNull();
  expect(profileStatus()?.textContent).toBe("プロフィールを確認中");
  await act(async () => finish({ id: "user-1", is_admin: isAdmin }));
  expect(profileStatus()).toBeNull();
  expect(adminLink()?.textContent ?? null).toBe(isAdmin ? "アプリ管理" : null);
});
it("プロフィールの応答がすぐ届いても一般ユーザーに管理画面の入口を表示しない", async () => {
  await renderSignedIn();
  expect(container.textContent).toContain("自分の投稿");
  expect(profileStatus()).toBeNull();
  expect(adminLink()).toBeNull();
});
it("別のアカウントへ切り替えたときに前の管理者の入口を表示しない", async () => {
  mock.apiFetch.mockResolvedValue({ id: "user-1", is_admin: true });
  await renderSignedIn();
  expect(adminLink()).not.toBeNull();

  let finish!: (profile: unknown) => void;
  mock.apiFetch.mockReturnValue(new Promise((resolve) => { finish = resolve; }));
  await signIn("SIGNED_IN", "user-2");
  expect(adminLink()).toBeNull();
  expect(profileStatus()?.textContent).toBe("プロフィールを確認中");

  await act(async () => finish({ id: "user-2", is_admin: false }));
  expect(profileStatus()).toBeNull();
  expect(adminLink()).toBeNull();
});
it("アプリ内の確認からこの端末をログアウトしてホームへ戻る", async () => {
  await renderSignedIn(); await logout();
  expect(mock.unsubscribe).toHaveBeenCalledWith("user-1", expect.any(Function));
  expect(mock.signOut).toHaveBeenCalledWith({ scope: "local" });
  expect(mock.replace).toHaveBeenCalledWith("/");
});
it("認証サーバーがエラーでも端末のセッションが消えていればホームへ戻る", async () => {
  mock.signOut.mockResolvedValue({ error: new Error("offline") });
  await renderSignedIn(); await logout();
  expect(mock.replace).toHaveBeenCalledWith("/");
});
it("セッションが残る失敗ではエラーを表示し、再試行できる", async () => {
  mock.signOut.mockResolvedValue({ error: new Error("failed") });
  mock.getSession.mockResolvedValue({ data: { session: { access_token: "token" } } });
  await renderSignedIn(); await logout();
  expect(container.querySelector('[role="alert"]')?.textContent).toContain("ログアウトできませんでした");
  expect([...container.querySelectorAll("button")].find((b) => b.textContent === "ログアウト")!.disabled).toBe(false);
  expect(mock.replace).not.toHaveBeenCalled();
});

// #504: 取得の失敗・未完了のあと、アプリを落とさなくても管理者の入口が出ること。
const networkError = () => Object.assign(new Error("通信に失敗しました"), { status: 0 });
const visible = async () => {
  await act(async () => document.dispatchEvent(new Event("visibilitychange")));
};
const unauthorized = () => Object.assign(new Error("unauthorized"), { status: 401 });

it.each(["SIGNED_IN", "INITIAL_SESSION"])(
  "ログイン直後のプロフィール取得が一度だけ401でも、再起動せず管理者の入口が出る (%s)",
  async (event) => {
    vi.useFakeTimers();
    try {
      mock.apiFetch.mockRejectedValueOnce(unauthorized()).mockResolvedValue({ id: "user-1", is_admin: true });
      await showPage(<div />);
      await vi.waitFor(() => expect(mock.onAuthStateChange).toHaveBeenCalledOnce());
      await signIn(event);
      await showPage();
      expect(adminLink()).toBeNull();
      const checking = profileStatus()?.textContent;

      await act(async () => { await vi.advanceTimersByTimeAsync(1_000); });
      expect(adminLink()?.textContent).toBe("アプリ管理");
      expect(checking).toBe("プロフィールを確認中");
      expect(profileStatus()).toBeNull();
      expect(mock.apiFetch).toHaveBeenCalledTimes(2);
    } finally { vi.useRealTimers(); }
  },
);

it("初回のプロフィール取得が通信エラーで失敗しても、待てば管理者の入口が出る", async () => {
  vi.useFakeTimers();
  try {
    mock.apiFetch.mockRejectedValueOnce(networkError()).mockResolvedValue({ id: "user-1", is_admin: true });
    await renderSignedIn();
    expect(adminLink()).toBeNull();
    expect(profileStatus()?.textContent).toBe("プロフィールを確認中");
    await act(async () => { await vi.advanceTimersByTimeAsync(30_000); });
    expect(adminLink()?.textContent).toBe("アプリ管理");
    expect(profileStatus()).toBeNull();
  } finally { vi.useRealTimers(); }
});
it("取得済みの管理者の入口は、復帰時の再取得が一時的に失敗しても消えない", async () => {
  mock.apiFetch.mockResolvedValue({ id: "user-1", is_admin: true });
  await renderSignedIn();
  expect(adminLink()).not.toBeNull();
  mock.apiFetch.mockRejectedValue(networkError());
  // 復帰時に認証 SDK が SIGNED_IN を出し直す。
  await signIn("SIGNED_IN");
  expect(adminLink()?.textContent).toBe("アプリ管理");
});
it("再試行を使い切っても、アプリの復帰で取り直して管理者の入口が出る", async () => {
  vi.useFakeTimers();
  try {
    mock.apiFetch.mockRejectedValue(networkError());
    await renderSignedIn();
    await act(async () => { await vi.advanceTimersByTimeAsync(120_000); });
    expect(adminLink()).toBeNull();
    expect(profileStatus()).toBeNull();
    const calls = mock.apiFetch.mock.calls.length;
    await act(async () => { await vi.advanceTimersByTimeAsync(600_000); });
    expect(mock.apiFetch.mock.calls.length).toBe(calls);
    mock.apiFetch.mockResolvedValue({ id: "user-1", is_admin: true });
    await visible();
    expect(adminLink()?.textContent).toBe("アプリ管理");
  } finally { vi.useRealTimers(); }
});
it("通信の復帰（online）でも未取得のプロフィールを取り直す", async () => {
  vi.useFakeTimers();
  try {
    mock.apiFetch.mockRejectedValue(networkError());
    await renderSignedIn();
    await act(async () => { await vi.advanceTimersByTimeAsync(120_000); });
    mock.apiFetch.mockResolvedValue({ id: "user-1", is_admin: true });
    await act(async () => window.dispatchEvent(new Event("online")));
    expect(adminLink()?.textContent).toBe("アプリ管理");
  } finally { vi.useRealTimers(); }
});
it("応答が返らないまま止まった取得は打ち切って取り直し、管理者の入口が出る", async () => {
  vi.useFakeTimers();
  try {
    mock.apiFetch.mockImplementationOnce((_path: string, init: { signal?: AbortSignal }) => new Promise((_resolve, reject) => {
      init.signal?.addEventListener("abort", () => reject(Object.assign(new Error("aborted"), { status: 0 })));
    })).mockResolvedValue({ id: "user-1", is_admin: true });
    await renderSignedIn();
    expect(profileStatus()?.textContent).toBe("プロフィールを確認中");
    await act(async () => { await vi.advanceTimersByTimeAsync(30_000); });
    expect(adminLink()?.textContent).toBe("アプリ管理");
  } finally { vi.useRealTimers(); }
});
it("マイページを開き直したときに、未取得のプロフィールを取り直す", async () => {
  vi.useFakeTimers();
  try {
    mock.apiFetch.mockRejectedValue(networkError());
    await renderSignedIn();
    await act(async () => { await vi.advanceTimersByTimeAsync(120_000); });
    expect(adminLink()).toBeNull();
    mock.apiFetch.mockResolvedValue({ id: "user-1", is_admin: true });
    await act(async () => root.render(<AuthProvider><ConfirmProvider><div /></ConfirmProvider></AuthProvider>));
    await act(async () => root.render(<AuthProvider><ConfirmProvider><MePage /></ConfirmProvider></AuthProvider>));
    expect(adminLink()?.textContent).toBe("アプリ管理");
  } finally { vi.useRealTimers(); }
});
it.each([
  ["通信エラー", networkError],
  ["401", unauthorized],
])("一般ユーザーには、%s・再試行・復帰のどの時点でも管理画面の入口を出さない", async (_error, profileError) => {
  vi.useFakeTimers();
  try {
    const seen: boolean[] = [];
    const observer = new MutationObserver(() => seen.push(Boolean(adminLink())));
    observer.observe(container, { childList: true, subtree: true });
    mock.apiFetch.mockRejectedValueOnce(profileError()).mockResolvedValue({ id: "user-1", is_admin: false });
    await render();
    await signIn("SIGNED_IN");
    expect(adminLink()).toBeNull();
    expect(profileStatus()?.textContent).toBe("プロフィールを確認中");
    await act(async () => { await vi.advanceTimersByTimeAsync(30_000); });
    expect(profileStatus()).toBeNull();
    mock.apiFetch.mockRejectedValue(networkError());
    await signIn("SIGNED_IN");
    await visible();
    await act(async () => window.dispatchEvent(new Event("online")));
    await act(async () => { await vi.advanceTimersByTimeAsync(120_000); });
    observer.disconnect();
    expect(seen.some(Boolean)).toBe(false);
    expect(adminLink()).toBeNull();
  } finally { vi.useRealTimers(); }
});
it("ログイン時に401が続く場合は1回で再試行を打ち切り、復帰や通信回復でも繰り返さない", async () => {
  vi.useFakeTimers();
  try {
    mock.apiFetch.mockRejectedValue(unauthorized());
    await renderSignedIn();
    expect(profileStatus()?.textContent).toBe("プロフィールを確認中");
    await act(async () => { await vi.advanceTimersByTimeAsync(120_000); });
    await visible();
    await act(async () => window.dispatchEvent(new Event("pageshow")));
    await act(async () => window.dispatchEvent(new Event("online")));
    await act(async () => { await vi.advanceTimersByTimeAsync(120_000); });
    expect(mock.apiFetch).toHaveBeenCalledTimes(2);
    expect(adminLink()).toBeNull();
    expect(profileStatus()).toBeNull();
  } finally { vi.useRealTimers(); }
});
it("コーチングが閉じている間はコーチングのセクションを出さない", async () => {
  await renderSignedIn();
  expect(container.textContent).not.toContain("コーチングチケット");
  expect(container.querySelector('nav a[href="/me/coaching/tickets"]')).toBeNull();
});

it("コーチングを開けている間は旧ハブと同じ順で4リンクのセクションを出す", async () => {
  flags.coaching = true;
  await renderSignedIn();
  const section = [...container.querySelectorAll("section")].find((s) => s.querySelector("h2")?.textContent === "コーチング");
  expect(section).not.toBeUndefined();
  expect([...section!.querySelectorAll("a")].map((a) => [a.getAttribute("href"), a.textContent])).toEqual([
    ["/me/coaching/tickets", "コーチングチケット"],
    ["/me/coaching/requests", "依頼した動画添削"],
    ["/coaching/support-consults", "1ヶ月プランの無料相談"],
    ["/me/coaching/personal", "申し込んだサポートプラン"],
  ]);
});

it("表示言語の項目はマイページの設定一覧に出さない（運営者決定 2026-11-13）", async () => {
  await renderSignedIn();
  expect(container.querySelector('nav a[href="/me/language"]')).toBeNull();
  expect(container.querySelector('[role="radiogroup"][lang], [role="radio"][lang]')).toBeNull();
  expect(container.textContent).not.toContain("表示言語");
});

it.each([
  { friends: 0, granted: false, label: "あと2人で無料チケット" },
  { friends: 1, granted: false, label: "あと1人で無料チケット" },
  { friends: 2, granted: true, label: "無料チケットをプレゼントしました" },
])("マイページに紹介IDと進捗を表示する ($friends人)", async ({ friends, granted, label }) => {
  mock.referral.mockResolvedValue({ referral: {
    code: "K7M3QX", active: !granted, friend_count: friends,
    granted_at: granted ? "2026-11-14T09:00:00Z" : null,
    expires_at: granted ? "2027-05-14T09:00:00Z" : null,
  } });
  await renderSignedIn();
  expect(container.textContent).toContain("K7M3QX");
  expect(container.textContent).toContain(label);
  if (granted) {
    expect(container.textContent).toContain("付与日");
    expect(container.textContent).toContain("有効期限");
    expect(container.textContent).toContain("付与された本人だけが使えます／払戻しはできません");
    expect(container.textContent).toContain("このIDは終了しました。紹介IDを使わずに購入すると、新しい紹介IDが発行されます。");
  }
  expect(container.textContent).toContain("購入すると、紹介IDが発行されます。そのIDで友達2人が購入すると、紹介者に無料チケットがプレゼントされます。");
});

it("紹介IDをまだ持たない人には、マイページで発行条件を案内する（本体 #686）", async () => {
  mock.referral.mockResolvedValue({ referral: null });
  await renderSignedIn();
  expect(container.textContent).toContain("友達2人を紹介すると、1回無料！");
  expect(container.textContent).toContain("紹介IDを使わずに購入すると、あなたの紹介IDが発行されます。");
});
