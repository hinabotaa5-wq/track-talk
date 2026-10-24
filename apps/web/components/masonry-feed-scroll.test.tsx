/** @vitest-environment jsdom */
import { act } from "react";
import { createRoot, type Root } from "react-dom/client";
import { afterEach, beforeEach, expect, it, vi } from "vitest";
import { MasonryFeed } from "./masonry-feed";

let container: HTMLDivElement;
let root: Root;
let frames: FrameRequestCallback[];
beforeEach(() => {
  (globalThis as typeof globalThis & { IS_REACT_ACT_ENVIRONMENT: boolean }).IS_REACT_ACT_ENVIRONMENT = true;
  frames = [];
  vi.stubGlobal("requestAnimationFrame", (callback: FrameRequestCallback) => { frames.push(callback); return frames.length; });
  vi.stubGlobal("cancelAnimationFrame", vi.fn());
  vi.stubGlobal("ResizeObserver", class { observe() {} disconnect() {} });
  vi.spyOn(HTMLElement.prototype, "clientWidth", "get").mockReturnValue(300);
  vi.spyOn(HTMLElement.prototype, "scrollWidth", "get").mockReturnValue(1200);
  container = document.createElement("div"); document.body.appendChild(container); root = createRoot(container);
});
afterEach(async () => {
  await act(async () => root.unmount()); container.remove(); vi.restoreAllMocks(); vi.unstubAllGlobals();
});
async function render(onEndReached: () => void) {
  await act(async () => root.render(<MasonryFeed ratios={[1]} onEndReached={onEndReached}><a href="/post">投稿</a></MasonryFeed>));
  await act(async () => { const pending = frames.splice(0); pending.forEach((callback) => callback(0)); });
  return container.querySelector('[role="region"]') as HTMLDivElement;
}
it("右端が近くなってから追加取得を要求する", async () => {
  const next = vi.fn();
  const track = await render(next);
  expect(next).not.toHaveBeenCalled();
  track.scrollLeft = 400; track.dispatchEvent(new Event("scroll"));
  expect(next).not.toHaveBeenCalled();
  track.scrollLeft = 800; track.dispatchEvent(new Event("scroll"));
  expect(next).toHaveBeenCalledTimes(1);
});
it("最初の投稿だけで画面が埋まらない場合は、続きを補う", async () => {
  vi.spyOn(HTMLElement.prototype, "scrollWidth", "get").mockReturnValue(300);
  const next = vi.fn();
  await render(next);
  expect(next).toHaveBeenCalledTimes(1);
});

it("詳細を開いて一覧を作り直しても横位置を復元する", async () => {
  vi.spyOn(HTMLElement.prototype, "offsetLeft", "get").mockImplementation(function (this: HTMLElement) { return Number.parseFloat(this.style.left) || 0; });
  vi.spyOn(HTMLElement.prototype, "offsetWidth", "get").mockImplementation(function (this: HTMLElement) { return Number.parseFloat(this.style.width) || 0; });
  const content = <MasonryFeed storageKey="restore-test" ratios={[1, 1, 1, 1, 1, 1]}>{Array.from({ length: 6 }, (_, i) => <a key={i} href={`/posts/${i}`}>投稿{i}</a>)}</MasonryFeed>;
  await act(async () => root.render(content));
  await act(async () => { frames.splice(0).forEach((callback) => callback(0)); });
  const track = container.querySelector('[role="region"]') as HTMLDivElement;
  track.scrollLeft = 400;
  track.dispatchEvent(new Event("scroll"));
  await act(async () => root.render(null));
  await act(async () => root.render(content));
  await act(async () => { frames.splice(0).forEach((callback) => callback(0)); });
  expect((container.querySelector('[role="region"]') as HTMLDivElement).scrollLeft).toBe(400);
});

it("同じフィードの高さを保存し、更新後もカードの形を保つ", async () => {
  sessionStorage.clear();
  const content = <MasonryFeed storageKey="fixed-layout" ratios={[1]}><a href="/fixed">投稿</a></MasonryFeed>;
  await act(async () => root.render(content));
  await act(async () => { frames.splice(0).forEach((callback) => callback(0)); });
  const height = container.querySelector<HTMLElement>(".masonry-feed-canvas")!.style.height;
  const width = container.querySelector<HTMLElement>(".masonry-feed-tile")!.style.width;
  await act(async () => root.render(null));
  vi.stubGlobal("innerHeight", 400);
  await act(async () => root.render(content));
  await act(async () => { frames.splice(0).forEach((callback) => callback(0)); });
  expect(container.querySelector<HTMLElement>(".masonry-feed-canvas")!.style.height).toBe(height);
  expect(container.querySelector<HTMLElement>(".masonry-feed-tile")!.style.width).toBe(width);
});

it("600件でもDOMを表示範囲に絞り、深い位置から戻っても復元する", async () => {
  const content = <MasonryFeed storageKey="large-feed" ratios={Array(600).fill(4 / 3)}>{Array.from({ length: 600 }, (_, i) => <a key={i} href={`/posts/${i}`}>投稿{i}</a>)}</MasonryFeed>;
  await act(async () => root.render(content));
  await act(async () => { frames.splice(0).forEach((callback) => callback(0)); });
  const track = container.querySelector('[role="region"]') as HTMLDivElement;
  expect(container.querySelectorAll("a").length).toBeLessThan(40);
  track.scrollLeft = 8000;
  await act(async () => { track.dispatchEvent(new Event("scroll")); frames.splice(0).forEach((callback) => callback(0)); });
  expect(container.querySelectorAll("a").length).toBeLessThan(40);
  expect(container.textContent).not.toContain("投稿0投稿1");
  await act(async () => root.render(null));
  await act(async () => root.render(content));
  await act(async () => { frames.splice(0).forEach((callback) => callback(0)); });
  expect((container.querySelector('[role="region"]') as HTMLDivElement).scrollLeft).toBe(8000);
});

it("配置の計算後に先頭へ戻す操作を、古い復元位置で上書きしない", async () => {
  const content = <MasonryFeed storageKey="scroll-interruption" ratios={Array(12).fill(1)}>{Array.from({ length: 12 }, (_, i) => <a key={i} href={`/posts/${i}`}>投稿{i}</a>)}</MasonryFeed>;
  await act(async () => root.render(content));
  await act(async () => { frames.splice(0).forEach((callback) => callback(0)); });
  const track = container.querySelector('[role="region"]') as HTMLDivElement;
  track.scrollLeft = 400;
  await act(async () => {
    window.dispatchEvent(new Event("resize"));
    frames.splice(0).forEach((callback) => callback(0));
    track.scrollLeft = 0;
    track.dispatchEvent(new Event("scroll"));
  });
  expect(track.scrollLeft).toBe(0);
});
