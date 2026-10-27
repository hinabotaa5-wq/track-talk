/** @vitest-environment jsdom */
import { act } from "react";
import { createRoot, type Root } from "react-dom/client";
import { renderToStaticMarkup } from "react-dom/server";
import { afterEach, beforeEach, expect, it, vi } from "vitest";
import { StartupSplash } from "./startup-splash";

const auth = vi.hoisted(() => ({ loading: true }));
vi.mock("@/components/auth-provider", () => ({ useAuth: () => auth }));

let root: Root;
let container: HTMLDivElement;
beforeEach(() => {
  (globalThis as typeof globalThis & { IS_REACT_ACT_ENVIRONMENT: boolean }).IS_REACT_ACT_ENVIRONMENT = true;
  vi.useFakeTimers();
  auth.loading = true;
  container = document.createElement("div");
  root = createRoot(container);
});
afterEach(async () => {
  await act(async () => root.unmount());
  vi.useRealTimers();
});

it("JavaScriptの実行前から白背景の中央にインラインロゴを描ける", () => {
  const html = renderToStaticMarkup(<StartupSplash />);
  container.innerHTML = html;
  const splash = container.querySelector<HTMLElement>("#startup-splash")!;
  expect(splash.style.position).toBe("fixed");
  expect(splash.style.background).toBe("rgb(255, 255, 255)");
  expect(splash.style.placeItems).toBe("center");
  expect(splash.querySelectorAll("svg path")).toHaveLength(3);
  expect(splash.querySelector("img")).toBeNull();
});

it("準備が済んだ次の描画で閉じ、その後の再読み込み状態では再表示しない", async () => {
  await act(async () => root.render(<StartupSplash />));
  expect(container.querySelector("#startup-splash")).not.toBeNull();
  await act(async () => vi.advanceTimersByTimeAsync(500));
  expect(container.querySelector("#startup-splash")).not.toBeNull();
  auth.loading = false;
  await act(async () => root.render(<StartupSplash />));
  await act(async () => vi.advanceTimersByTimeAsync(20));
  expect(container.querySelector("#startup-splash")).toBeNull();
  auth.loading = true;
  await act(async () => root.render(<StartupSplash />));
  expect(container.querySelector("#startup-splash")).toBeNull();
});

it("認証の応答がない場合もロゴを表示したまま閉じ込めない", async () => {
  await act(async () => root.render(<StartupSplash />));
  await act(async () => vi.advanceTimersByTimeAsync(10000));
  expect(container.querySelector("#startup-splash")).toBeNull();
});
