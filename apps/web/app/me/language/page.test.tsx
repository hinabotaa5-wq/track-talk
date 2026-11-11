// @vitest-environment jsdom
import { act } from "react";
import { createRoot, type Root } from "react-dom/client";
import { afterEach, beforeEach, expect, it, vi } from "vitest";
import { LocaleProvider } from "@/components/locale-provider";
import LanguagePage from "./page";

// マイページの設定一覧から開く「表示言語」の画面（本体 #536）。
const auth = vi.hoisted(() => ({
  session: {} as unknown,
  loading: false,
  profile: { locale: "ja", nickname: "ランナー", event: "100m" },
  token: "token",
  refreshProfile: vi.fn(),
}));
vi.mock("@/components/auth-provider", () => ({ useAuth: () => auth }));
const nav = vi.hoisted(() => ({ replace: vi.fn() }));
vi.mock("next/navigation", () => ({ usePathname: () => "/me/language", useRouter: () => ({ replace: nav.replace }) }));
vi.mock("@/lib/api", async (original) => ({ ...(await original<typeof import("@/lib/api")>()), apiFetch: vi.fn() }));

let root: Root;
let container: HTMLDivElement;
async function render() {
  await act(async () => root.render(<LocaleProvider><LanguagePage /></LocaleProvider>));
}
beforeEach(() => {
  (globalThis as typeof globalThis & { IS_REACT_ACT_ENVIRONMENT: boolean }).IS_REACT_ACT_ENVIRONMENT = true;
  auth.session = {};
  nav.replace.mockReset();
  container = document.createElement("div");
  document.body.appendChild(container);
  root = createRoot(container);
});
afterEach(async () => {
  await act(async () => root.unmount());
  container.remove();
  localStorage.clear();
});

it("見出しと、日本語／English の選択を出す", async () => {
  await render();
  expect(container.querySelector("h1")?.textContent).toBe("表示言語");
  const group = container.querySelector('[role="radiogroup"]');
  expect(group?.getAttribute("aria-labelledby")).toBe(container.querySelector("h1")?.id);
  expect([...group!.querySelectorAll('[role="radio"]')].map((b) => b.textContent)).toEqual(["日本語", "English"]);
});

it("未ログインならログイン画面へ送る", async () => {
  auth.session = null;
  await render();
  expect(nav.replace).toHaveBeenCalledWith("/login?next=/me/language");
});
