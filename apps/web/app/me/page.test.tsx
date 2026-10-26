/** @vitest-environment jsdom */
import { act } from "react";
import { createRoot, type Root } from "react-dom/client";
import { afterEach, beforeEach, expect, it, vi } from "vitest";
import { ConfirmProvider } from "@/components/confirm-dialog";
import MePage from "./page";
const mock = vi.hoisted(() => ({ signOut: vi.fn(), getSession: vi.fn(), unsubscribe: vi.fn(), replace: vi.fn(), refresh: vi.fn() }));
vi.mock("@/components/auth-provider", () => ({ useAuth: () => ({ session: { access_token: "token" }, token: "token", profile: null, loading: true, sessionLoading: false }) }));
vi.mock("@/lib/supabase", () => ({ createClient: () => ({ auth: { signOut: mock.signOut, getSession: mock.getSession } }) }));
vi.mock("@/lib/sign-out", () => ({ unsubscribePushOnThisDevice: mock.unsubscribe }));
vi.mock("next/navigation", () => ({ useRouter: () => ({ replace: mock.replace, refresh: mock.refresh }) }));
let root: Root, container: HTMLDivElement;
beforeEach(() => {
  (globalThis as { IS_REACT_ACT_ENVIRONMENT?: boolean }).IS_REACT_ACT_ENVIRONMENT = true;
  vi.clearAllMocks();
  mock.signOut.mockResolvedValue({ error: null });
  mock.unsubscribe.mockResolvedValue(undefined);
  mock.getSession.mockResolvedValue({ data: { session: null } });
  container = document.createElement("div"); document.body.appendChild(container); root = createRoot(container);
});
afterEach(async () => { await act(async () => root.unmount()); container.remove(); });
async function render() { await act(async () => root.render(<ConfirmProvider><MePage /></ConfirmProvider>)); }
async function logout() {
  await act(async () => [...container.querySelectorAll("button")].find((b) => b.textContent === "ログアウト")!.click());
  await act(async () => container.querySelector<HTMLButtonElement>('[role="alertdialog"] button')!.click());
}
it("プロフィールの取得待ちでもメニューを表示し、追加ガイドは独立ページに移動する", async () => {
  await render();
  expect(container.textContent).toContain("自分の投稿");
  expect(container.querySelector('a[href="/me/install"]')?.textContent).toContain("ホーム画面に追加して使う");
  expect(container.textContent).not.toContain("読み込み中");
});
it("アプリ内の確認からこの端末をログアウトしてホームへ戻る", async () => {
  await render(); await logout();
  expect(mock.unsubscribe).toHaveBeenCalledWith("token");
  expect(mock.signOut).toHaveBeenCalledWith({ scope: "local" });
  expect(mock.replace).toHaveBeenCalledWith("/");
});
it("認証サーバーがエラーでも端末のセッションが消えていればホームへ戻る", async () => {
  mock.signOut.mockResolvedValue({ error: new Error("offline") });
  await render(); await logout();
  expect(mock.replace).toHaveBeenCalledWith("/");
});
it("セッションが残る失敗ではエラーを表示し、再試行できる", async () => {
  mock.signOut.mockResolvedValue({ error: new Error("failed") });
  mock.getSession.mockResolvedValue({ data: { session: { access_token: "token" } } });
  await render(); await logout();
  expect(container.querySelector('[role="alert"]')?.textContent).toContain("ログアウトできませんでした");
  expect([...container.querySelectorAll("button")].find((b) => b.textContent === "ログアウト")!.disabled).toBe(false);
  expect(mock.replace).not.toHaveBeenCalled();
});
