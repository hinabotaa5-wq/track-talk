// @vitest-environment jsdom
import { act, type ReactNode } from "react";
import { createRoot, type Root } from "react-dom/client";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import { LocalePrompt } from "./locale-prompt";

// TEMPORARY（本体 #521 追加要件）のプロンプトのテスト。本体を外すときはこのファイルも消す。

const auth = vi.hoisted(() => ({ session: null as unknown, sessionLoading: false }));
vi.mock("@/components/auth-provider", () => ({
  useAuth: () => ({ session: auth.session, sessionLoading: auth.sessionLoading }),
}));
const nav = vi.hoisted(() => ({ pathname: "/" }));
vi.mock("next/navigation", () => ({ usePathname: () => nav.pathname }));
vi.mock("next/link", () => ({
  default: ({ href, children, ...props }: { href: string; children: ReactNode }) => (
    <a href={href} {...props}>{children}</a>
  ),
}));

// アクセス元の国コード（Netlify の端が /geo-country で返すもの）の差し替え。既定は US。
const geo = vi.hoisted(() => ({ country: "US" as string | null, fail: false, calls: 0 }));
function mockGeoFetch() {
  vi.stubGlobal("fetch", vi.fn(async (input: RequestInfo | URL) => {
    if (String(input) !== "/geo-country") throw new Error(`unexpected fetch ${String(input)}`);
    geo.calls += 1;
    if (geo.fail) throw new TypeError("network");
    return new Response(JSON.stringify({ country: geo.country }), { headers: { "Content-Type": "application/json" } });
  }));
}

let root: Root;
let container: HTMLDivElement;
beforeEach(() => {
  // jsdom は dialog のトップレイヤーを実装していない。
  Object.defineProperties(HTMLDialogElement.prototype, {
    showModal: { configurable: true, value(this: HTMLDialogElement) { this.open = true; } },
    close: { configurable: true, value(this: HTMLDialogElement) { this.open = false; } },
  });
  (globalThis as typeof globalThis & { IS_REACT_ACT_ENVIRONMENT: boolean }).IS_REACT_ACT_ENVIRONMENT = true;
  localStorage.clear();
  auth.session = null;
  auth.sessionLoading = false;
  nav.pathname = "/";
  geo.country = "US";
  geo.fail = false;
  geo.calls = 0;
  mockGeoFetch();
  container = document.createElement("div");
  document.body.appendChild(container);
  root = createRoot(container);
});
afterEach(async () => {
  await act(async () => root.unmount());
  container.remove();
  localStorage.clear();
  vi.unstubAllGlobals();
  Reflect.deleteProperty(HTMLDialogElement.prototype, "showModal");
  Reflect.deleteProperty(HTMLDialogElement.prototype, "close");
});

describe("未ログイン時の言語選択プロンプト", () => {
  it("US からの未ログインの初回は出て、英語を選ぶと保存して閉じる", async () => {
    await act(async () => root.render(<LocalePrompt />));
    expect(container.textContent).toContain("Choose your language");
    await act(async () => {
      container.querySelectorAll("button")[1].dispatchEvent(new MouseEvent("click", { bubbles: true }));
    });
    expect(localStorage.getItem("tracktalk-locale")).toBe("en");
    expect(localStorage.getItem("tracktalk:locale-prompted")).toBe("1");
    expect(container.textContent).not.toContain("Choose your language");
  });

  it("英語固定の海外コーチ向け紹介ページ（/for-coaches）では出さない", async () => {
    nav.pathname = "/for-coaches";
    await act(async () => root.render(<LocalePrompt />));
    expect(container.textContent).not.toContain("Choose your language");
    expect(localStorage.getItem("tracktalk:locale-prompted")).toBeNull();
  });

  it("答えたあとは US でも出ない（国も問い合わせない）", async () => {
    localStorage.setItem("tracktalk:locale-prompted", "1");
    await act(async () => root.render(<LocalePrompt />));
    expect(container.textContent).not.toContain("Choose your language");
    expect(geo.calls).toBe(0);
  });

  it("ログイン中は US でも出ない（国も問い合わせない）", async () => {
    auth.session = {};
    await act(async () => root.render(<LocalePrompt />));
    expect(container.textContent).not.toContain("Choose your language");
    expect(geo.calls).toBe(0);
  });

  it.each(["JP", "GB", "CA"])("US 以外（%s）からは出さず、回答記録も付けない", async (country) => {
    geo.country = country;
    await act(async () => root.render(<LocalePrompt />));
    expect(geo.calls).toBe(1);
    expect(container.textContent).not.toContain("Choose your language");
    expect(localStorage.getItem("tracktalk:locale-prompted")).toBeNull();
    expect(localStorage.getItem("tracktalk-locale")).toBeNull();
  });

  it("国コードが取れないときは出さない", async () => {
    geo.country = null;
    await act(async () => root.render(<LocalePrompt />));
    expect(geo.calls).toBe(1);
    expect(container.textContent).not.toContain("Choose your language");
  });

  it("国の問い合わせに失敗したときは出さない", async () => {
    geo.fail = true;
    await act(async () => root.render(<LocalePrompt />));
    expect(geo.calls).toBe(1);
    expect(container.textContent).not.toContain("Choose your language");
  });

  it("国の問い合わせが 404 などのときは出さない", async () => {
    vi.stubGlobal("fetch", vi.fn(async () => new Response("not found", { status: 404 })));
    await act(async () => root.render(<LocalePrompt />));
    expect(container.textContent).not.toContain("Choose your language");
  });

  it("選ばないと閉じられない（閉じる導線なし・Esc でも閉じない）", async () => {
    await act(async () => root.render(<LocalePrompt />));
    // 閉じるためのボタンはない。
    expect(container.textContent).not.toContain("閉じる");
    // Esc（cancel イベント）でも閉じず、回答記録も付かない。
    await act(async () => {
      container.querySelector("dialog")?.dispatchEvent(new Event("cancel", { bubbles: true, cancelable: true }));
    });
    expect(container.textContent).toContain("Choose your language");
    expect(localStorage.getItem("tracktalk:locale-prompted")).toBeNull();
  });
});
