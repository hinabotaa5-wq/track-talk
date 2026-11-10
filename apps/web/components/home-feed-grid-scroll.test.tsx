/** @vitest-environment jsdom */
import { act } from "react";
import { createRoot, type Root } from "react-dom/client";
import { afterEach, beforeEach, expect, it, vi } from "vitest";
import { HomeFeedGrid } from "./home-feed-grid";

let container: HTMLDivElement;
let root: Root;
let frames: FrameRequestCallback[];
let resizeObservers: Map<Element, () => void>;
beforeEach(() => {
  (globalThis as typeof globalThis & { IS_REACT_ACT_ENVIRONMENT: boolean }).IS_REACT_ACT_ENVIRONMENT = true;
  frames = [];
  vi.stubGlobal("requestAnimationFrame", (callback: FrameRequestCallback) => { frames.push(callback); return frames.length; });
  vi.stubGlobal("cancelAnimationFrame", vi.fn());
  resizeObservers = new Map();
  vi.stubGlobal("ResizeObserver", class {
    constructor(private readonly callback: () => void) {}
    observe(element: Element) { resizeObservers.set(element, this.callback); }
    disconnect() { resizeObservers.clear(); }
  });
  vi.spyOn(HTMLElement.prototype, "clientWidth", "get").mockReturnValue(300);
  vi.spyOn(HTMLElement.prototype, "scrollWidth", "get").mockReturnValue(1200);
  container = document.createElement("div"); document.body.appendChild(container); root = createRoot(container);
});
afterEach(async () => {
  await act(async () => root.unmount()); container.remove(); vi.restoreAllMocks(); vi.unstubAllGlobals();
});
async function render(onEndReached: () => void) {
  await act(async () => root.render(<HomeFeedGrid onEndReached={onEndReached}><a href="/post">投稿</a></HomeFeedGrid>));
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
  const content = <HomeFeedGrid storageKey="restore-test">{Array.from({ length: 6 }, (_, i) => <a key={i} href={`/posts/${i}`}>投稿{i}</a>)}</HomeFeedGrid>;
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

it("ヘッダーに帯が後から出ても、一覧を下部メニューの手前に収める", async () => {
  let headerHeight = 64;
  let navTop = 700;
  vi.spyOn(HTMLElement.prototype, "getBoundingClientRect").mockImplementation(function (this: HTMLElement) {
    if (this.tagName === "HEADER") return new DOMRect(0, 0, 300, headerHeight);
    if (this.tagName === "NAV") return new DOMRect(0, navTop, 300, 56);
    if (this.classList.contains("home-feed-grid-strip")) return new DOMRect(0, headerHeight + 60, 300, 0);
    return new DOMRect();
  });
  const header = document.createElement("header");
  const nav = document.createElement("nav");
  nav.setAttribute("data-main-nav", "");
  nav.style.position = "fixed";
  container.before(header);
  container.after(nav);
  try {
    await render(vi.fn());
    const canvas = container.querySelector<HTMLElement>(".home-feed-grid-canvas")!;
    expect(canvas.style.height).toBe("568px");

    headerHeight += 44;
    await act(async () => {
      resizeObservers.get(header)?.();
      frames.splice(0).forEach((callback) => callback(0));
    });
    expect(canvas.style.height).toBe("524px");

    navTop -= 60;
    await act(async () => {
      window.dispatchEvent(new Event("resize"));
      frames.splice(0).forEach((callback) => callback(0));
    });
    expect(canvas.style.height).toBe("464px");
  } finally {
    header.remove();
    nav.remove();
  }
});

it("ヘッダーに帯が後から出ても段数は3のまま変わらない", async () => {
  let headerHeight = 64;
  const navTop = 700;
  vi.spyOn(HTMLElement.prototype, "getBoundingClientRect").mockImplementation(function (this: HTMLElement) {
    if (this.tagName === "HEADER") return new DOMRect(0, 0, 300, headerHeight);
    if (this.tagName === "NAV") return new DOMRect(0, navTop, 300, 56);
    if (this.classList.contains("home-feed-grid-strip")) return new DOMRect(0, headerHeight + 60, 300, 0);
    return new DOMRect();
  });
  const header = document.createElement("header");
  const nav = document.createElement("nav");
  nav.setAttribute("data-main-nav", "");
  nav.style.position = "fixed";
  container.before(header);
  container.after(nav);
  try {
    const content = <HomeFeedGrid>{Array.from({ length: 6 }, (_, i) => <a key={i} href={`/posts/${i}`}>投稿{i}</a>)}</HomeFeedGrid>;
    await act(async () => root.render(content));
    await act(async () => { frames.splice(0).forEach((callback) => callback(0)); });
    const tops = () => new Set(Array.from(container.querySelectorAll<HTMLElement>(".home-feed-grid-tile")).map((tile) => tile.style.top));
    expect(tops().size).toBe(3);

    headerHeight += 44;
    await act(async () => {
      resizeObservers.get(header)?.();
      frames.splice(0).forEach((callback) => callback(0));
    });
    expect(tops().size).toBe(3);
  } finally {
    header.remove();
    nav.remove();
  }
});

it("600件でもDOMを表示範囲に絞り、深い位置から戻っても復元する", async () => {
  const content = <HomeFeedGrid storageKey="large-feed">{Array.from({ length: 600 }, (_, i) => <a key={i} href={`/posts/${i}`}>投稿{i}</a>)}</HomeFeedGrid>;
  await act(async () => root.render(content));
  await act(async () => { frames.splice(0).forEach((callback) => callback(0)); });
  const track = container.querySelector('[role="region"]') as HTMLDivElement;
  expect(container.querySelectorAll("a").length).toBeLessThan(50);
  track.scrollLeft = 8000;
  await act(async () => { track.dispatchEvent(new Event("scroll")); frames.splice(0).forEach((callback) => callback(0)); });
  expect(container.querySelectorAll("a").length).toBeLessThan(50);
  expect(container.textContent).not.toContain("投稿0投稿1");
  await act(async () => root.render(null));
  await act(async () => root.render(content));
  await act(async () => { frames.splice(0).forEach((callback) => callback(0)); });
  expect((container.querySelector('[role="region"]') as HTMLDivElement).scrollLeft).toBe(8000);
});

it("配置の計算後に先頭へ戻す操作を、古い復元位置で上書きしない", async () => {
  const content = <HomeFeedGrid storageKey="scroll-interruption">{Array.from({ length: 12 }, (_, i) => <a key={i} href={`/posts/${i}`}>投稿{i}</a>)}</HomeFeedGrid>;
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
