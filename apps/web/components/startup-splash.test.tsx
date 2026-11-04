/** @vitest-environment jsdom */
import { act } from "react";
import { createRoot, type Root } from "react-dom/client";
import { renderToStaticMarkup } from "react-dom/server";
import { afterEach, beforeEach, expect, it, vi } from "vitest";
import { StartupSplash } from "./startup-splash";

type TestProfile = { nickname: string | null; event: string | null; overseas_consent?: boolean } | null;
const auth = vi.hoisted(() => ({ loading: true, profile: null as TestProfile }));
const nav = vi.hoisted(() => ({ pathname: "/" }));
vi.mock("@/components/auth-provider", () => ({ useAuth: () => auth }));
vi.mock("next/navigation", () => ({ usePathname: () => nav.pathname }));

let root: Root;
let container: HTMLDivElement;
beforeEach(() => {
  (globalThis as typeof globalThis & { IS_REACT_ACT_ENVIRONMENT: boolean }).IS_REACT_ACT_ENVIRONMENT = true;
  vi.useFakeTimers();
  auth.loading = true;
  auth.profile = null;
  nav.pathname = "/";
  container = document.createElement("div");
  root = createRoot(container);
});
afterEach(async () => {
  await act(async () => root.unmount());
  vi.useRealTimers();
});

it("JavaScriptの実行前から画面の背景色(既定は白)の中央にインラインロゴを描ける", () => {
  const html = renderToStaticMarkup(<StartupSplash />);
  container.innerHTML = html;
  const splash = container.querySelector<HTMLElement>("#startup-splash")!;
  expect(splash.style.position).toBe("fixed");
  expect(splash.style.background).toBe("var(--background, #ffffff)");
  expect(splash.style.placeItems).toBe("center");
  expect(splash.querySelectorAll("svg path")).toHaveLength(3);
  expect(splash.querySelector("img")).toBeNull();
});

it("準備が済んだらフェードしてから閉じ、その後の再読み込み状態では再表示しない", async () => {
  await act(async () => root.render(<StartupSplash />));
  expect(container.querySelector("#startup-splash")).not.toBeNull();
  await act(async () => vi.advanceTimersByTimeAsync(500));
  expect(container.querySelector("#startup-splash")).not.toBeNull();
  auth.loading = false;
  await act(async () => root.render(<StartupSplash />));
  await act(async () => vi.advanceTimersByTimeAsync(20));
  // 急に消さず、フェード中の状態を経由する。
  expect(container.querySelector("#startup-splash")?.getAttribute("data-state")).toBe("leaving");
  await act(async () => vi.advanceTimersByTimeAsync(400));
  expect(container.querySelector("#startup-splash")).toBeNull();
  auth.loading = true;
  await act(async () => root.render(<StartupSplash />));
  expect(container.querySelector("#startup-splash")).toBeNull();
});

it("認証の応答がない場合もロゴを表示したまま閉じ込めない", async () => {
  await act(async () => root.render(<StartupSplash />));
  await act(async () => vi.advanceTimersByTimeAsync(10000));
  expect(container.querySelector("#startup-splash")?.getAttribute("data-state")).toBe("leaving");
  await act(async () => vi.advanceTimersByTimeAsync(400));
  expect(container.querySelector("#startup-splash")).toBeNull();
});

it("ニックネームを決める前の人は、決める画面に移るまでホームを見せずにロゴを出し続ける", async () => {
  await act(async () => root.render(<StartupSplash />));
  auth.loading = false;
  auth.profile = { nickname: null, event: null, overseas_consent: true };
  await act(async () => root.render(<StartupSplash />));
  await act(async () => vi.advanceTimersByTimeAsync(1000));
  expect(container.querySelector("#startup-splash")?.getAttribute("data-state")).toBe("shown");
  nav.pathname = "/me/profile";
  await act(async () => root.render(<StartupSplash />));
  await act(async () => vi.advanceTimersByTimeAsync(20));
  expect(container.querySelector("#startup-splash")?.getAttribute("data-state")).toBe("leaving");
  await act(async () => vi.advanceTimersByTimeAsync(400));
  expect(container.querySelector("#startup-splash")).toBeNull();
});

it("同意を記録するあいだも、ホームを見せずにロゴを出し続ける", async () => {
  await act(async () => root.render(<StartupSplash />));
  auth.loading = false;
  auth.profile = { nickname: null, event: null, overseas_consent: false };
  nav.pathname = "/me/profile";
  await act(async () => root.render(<StartupSplash />));
  await act(async () => vi.advanceTimersByTimeAsync(1000));
  expect(container.querySelector("#startup-splash")?.getAttribute("data-state")).toBe("shown");
  // 記録できてプロフィールを読み直すと、決める画面のまま閉じる。
  auth.profile = { nickname: null, event: null, overseas_consent: true };
  await act(async () => root.render(<StartupSplash />));
  await act(async () => vi.advanceTimersByTimeAsync(20));
  expect(container.querySelector("#startup-splash")?.getAttribute("data-state")).toBe("leaving");
});

it("決める画面に移れないままでも、10秒で閉じて閉じ込めない", async () => {
  auth.loading = false;
  auth.profile = { nickname: null, event: null, overseas_consent: true };
  await act(async () => root.render(<StartupSplash />));
  await act(async () => vi.advanceTimersByTimeAsync(10000));
  expect(container.querySelector("#startup-splash")?.getAttribute("data-state")).toBe("leaving");
});
