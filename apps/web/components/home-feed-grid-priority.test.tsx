/** @vitest-environment jsdom */
import { act, createElement, type ReactNode } from "react";
import { createRoot, type Root } from "react-dom/client";
import { afterEach, beforeEach, expect, it, vi } from "vitest";
import { HomeFeedGrid } from "./home-feed-grid";
import { FeedCard } from "./feed-card";
import type { Post } from "@/lib/types";

vi.mock("next/link", () => ({
  default: ({ children, href, ...props }: { children: ReactNode; href: string }) => createElement("a", { href, ...props }, children),
  useLinkStatus: () => ({ pending: false }),
}));

function videoPost(i: number) {
  return {
    id: `priority-post-${i}`,
    title: `投稿${i}`,
    mine: false,
    like_count: 0,
    comment_count: 0,
    liked_by_me: false,
    saved_by_me: false,
    kind: "video",
    playback_url: "https://example.com/v.mp4",
    thumbnail_url: `https://example.com/t${i}.jpg`,
    created_at: "2026-10-12T00:00:00Z",
    updated_at: "2026-10-12T00:00:00Z",
  } as Post;
}

let container: HTMLDivElement;
let root: Root;
let frames: FrameRequestCallback[];
beforeEach(() => {
  (globalThis as typeof globalThis & { IS_REACT_ACT_ENVIRONMENT: boolean }).IS_REACT_ACT_ENVIRONMENT = true;
  frames = [];
  vi.stubGlobal("requestAnimationFrame", (callback: FrameRequestCallback) => { frames.push(callback); return frames.length; });
  vi.stubGlobal("cancelAnimationFrame", vi.fn());
  vi.stubGlobal("ResizeObserver", class {
    observe() {}
    disconnect() {}
  });
  // スマホ幅相当で計測させる。
  vi.spyOn(HTMLElement.prototype, "clientWidth", "get").mockReturnValue(390);
  vi.spyOn(HTMLElement.prototype, "scrollWidth", "get").mockReturnValue(4000);
  container = document.createElement("div"); document.body.appendChild(container); root = createRoot(container);
});
afterEach(async () => {
  await act(async () => root.unmount()); container.remove(); vi.restoreAllMocks(); vi.unstubAllGlobals();
});

function tileImages() {
  return Array.from(container.querySelectorAll<HTMLElement>(".home-feed-grid-tile")).map((tile) => ({
    index: Number(tile.dataset.index),
    img: tile.querySelector("img"),
  }));
}

it("先頭の画面のタイルだけ即時読み込み＋優先指定にし、残りはlazyのままにする", async () => {
  const content = <HomeFeedGrid>{Array.from({ length: 30 }, (_, i) => <FeedCard key={i} post={videoPost(i)} />)}</HomeFeedGrid>;
  await act(async () => root.render(content));
  await act(async () => { frames.splice(0).forEach((callback) => callback(0)); });

  // 横に1列分スクロールし、先読み余白で載ったタイルまで描画させる。
  const track = container.querySelector('[role="region"]') as HTMLDivElement;
  track.scrollLeft = 340;
  await act(async () => { track.dispatchEvent(new Event("scroll")); frames.splice(0).forEach((callback) => callback(0)); });

  const tiles = tileImages();
  expect(tiles.length).toBeGreaterThan(6);
  const eager = tiles.filter(({ img }) => img?.getAttribute("loading") === "eager");
  const lazy = tiles.filter(({ img }) => img?.getAttribute("loading") === "lazy");
  // 先頭の画面分は即時＋優先、その先の先読み分は遅延のまま。
  expect(eager.length).toBeGreaterThan(0);
  expect(lazy.length).toBeGreaterThan(0);
  for (const { img } of eager) expect(img?.getAttribute("fetchpriority")).toBe("high");
  for (const { img } of lazy) expect(img?.getAttribute("fetchpriority")).toBeNull();
  const maxEager = Math.max(...eager.map(({ index }) => index));
  const minLazy = Math.min(...lazy.map(({ index }) => index));
  expect(maxEager).toBeLessThan(minLazy);
  // 先頭タイルは必ず即時読み込み。
  expect(tiles.find(({ index }) => index === 0)?.img?.getAttribute("loading")).toBe("eager");
});

it("サムネイルのURLは変えない", async () => {
  const content = <HomeFeedGrid>{Array.from({ length: 12 }, (_, i) => <FeedCard key={i} post={videoPost(i)} />)}</HomeFeedGrid>;
  await act(async () => root.render(content));
  await act(async () => { frames.splice(0).forEach((callback) => callback(0)); });
  const first = container.querySelector(".home-feed-grid-tile img");
  expect(first?.getAttribute("src")).toBe("https://example.com/t0.jpg");
});
