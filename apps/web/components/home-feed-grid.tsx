"use client";

import { Children, useEffect, useLayoutEffect, useRef, useState, type ReactNode } from "react";
import { createGridLayout, gridTilePosition, visibleGridIndices, type GridLayout } from "@/lib/home-feed-grid";
import { BoundedCache } from "@/lib/bounded-cache";

type FeedPosition = { left: number; key?: string; offset: number };
const positions = new BoundedCache<FeedPosition>(20, 30 * 60_000);

export function HomeFeedGrid({ children, onEndReached, onVisibleIndices, storageKey }: {
  children: ReactNode; onEndReached?: () => void; onVisibleIndices?: (indices: number[]) => void; storageKey?: string;
}) {
  const trackRef = useRef<HTMLDivElement>(null);
  const items = Children.toArray(children);
  const keys = items.map((item, i) => item && typeof item === "object" && "key" in item ? String(item.key) : String(i));
  const keysRef = useRef(keys);
  const [layout, setLayout] = useState<GridLayout | null>(null);
  const layoutRef = useRef(layout);
  const [viewport, setViewport] = useState({ left: 0, width: 0 });
  const pendingPosition = useRef<FeedPosition | undefined>(undefined);
  const visibleRef = useRef(onVisibleIndices);
  const nextRef = useRef(onEndReached);
  const [focusedIndex, setFocusedIndex] = useState<number>(NaN);
  useLayoutEffect(() => {
    keysRef.current = keys;
    layoutRef.current = layout;
    visibleRef.current = onVisibleIndices;
    nextRef.current = onEndReached;
  });
  const scheduleRef = useRef<() => void>(() => {});
  const count = items.length;

  useEffect(() => {
    const track = trackRef.current;
    if (!track) return;
    pendingPosition.current = storageKey ? positions.get(storageKey) : undefined;
    let frame = 0, scrollFrame = 0;
    const canvas = track.firstElementChild as HTMLElement;
    const header = document.querySelector("header");
    const filters = document.querySelector<HTMLElement>(".home-filter-anchor");
    const navs = Array.from(document.querySelectorAll<HTMLElement>('nav[aria-label="メイン"]'));
    function savePosition(event?: Event) {
      if (!storageKey || pendingPosition.current) return;
      const nodes = Array.from(canvas.children) as HTMLElement[];
      const clicked = event?.target instanceof Element ? event.target.closest(".home-feed-grid-tile") as HTMLElement | null : null;
      const anchor = clicked ?? nodes.find((node) => node.offsetLeft + node.offsetWidth > track!.scrollLeft);
      positions.set(storageKey, { left: track!.scrollLeft, key: anchor?.dataset.key, offset: (anchor?.offsetLeft ?? 0) - track!.scrollLeft });
    }
    function layoutNow() {
      const width = Math.round(track!.clientWidth);
      if (width <= 0) return;
      const bottomNav = navs.find((nav) => getComputedStyle(nav).position === "fixed" && nav.getBoundingClientRect().height > 0);
      if (header) filters?.style.setProperty("--home-header-height", `${header.getBoundingClientRect().height}px`);
      // 帯の表示などで一覧の開始位置が変わったら、下部メニューまでの高さを測り直す。
      const bottom = bottomNav?.getBoundingClientRect().top ?? window.innerHeight;
      const available = Math.max(3, Math.floor(bottom - (track!.getBoundingClientRect().top + window.scrollY) - 8));
      const columns = Number(getComputedStyle(track!).getPropertyValue("--home-feed-columns")) || 2;
      const ids = keysRef.current;
      const result = createGridLayout(ids.length, width, available, columns);
      const old = layoutRef.current;
      // 高さや幅が変わったら、表示中の列を基準に横位置を保つ。
      if (!pendingPosition.current && old && track!.scrollLeft > 0
        && (old.rows !== result.rows || old.tileWidth !== result.tileWidth)) {
        const index = visibleGridIndices(old, track!.scrollLeft, 1, 0)[0];
        if (index !== undefined) {
          pendingPosition.current = { left: track!.scrollLeft, key: ids[index], offset: gridTilePosition(old, index).left - track!.scrollLeft };
        }
      }
      setLayout(result);
    }
    function schedule() { cancelAnimationFrame(frame); frame = requestAnimationFrame(layoutNow); }
    scheduleRef.current = schedule;
    function onScroll() {
      // レイアウト計算と描画の間に移動した場合は、新しい操作を優先する。
      pendingPosition.current = undefined;
      savePosition();
      cancelAnimationFrame(scrollFrame);
      scrollFrame = requestAnimationFrame(() => setViewport({ left: track!.scrollLeft, width: track!.clientWidth }));
      const remaining = track!.scrollWidth - track!.scrollLeft - track!.clientWidth;
      if (remaining <= Math.max(160, track!.clientWidth * 0.5) && track!.scrollLeft > 0) nextRef.current?.();
    }
    function wheel(event: WheelEvent) {
      if (!event.cancelable || event.ctrlKey || event.metaKey || event.shiftKey
        || !window.matchMedia("(hover: hover) and (pointer: fine)").matches || Math.abs(event.deltaX) >= Math.abs(event.deltaY)) return;
      const unit = event.deltaMode === WheelEvent.DOM_DELTA_LINE ? 16 : event.deltaMode === WheelEvent.DOM_DELTA_PAGE ? track!.clientWidth : 1;
      const delta = event.deltaY * unit, max = track!.scrollWidth - track!.clientWidth;
      if (max <= 0 || (delta > 0 && track!.scrollLeft >= max - 1) || (delta < 0 && track!.scrollLeft <= 1)) return;
      event.preventDefault();
      track!.scrollLeft = Math.max(0, Math.min(max, track!.scrollLeft + delta));
    }
    function keyDown(event: KeyboardEvent) {
      if (event.key !== "ArrowLeft" && event.key !== "ArrowRight") return;
      event.preventDefault();
      track!.scrollLeft = Math.max(0, Math.min(track!.scrollWidth - track!.clientWidth, track!.scrollLeft + Math.max(120, track!.clientWidth * 0.72) * (event.key === "ArrowRight" ? 1 : -1)));
    }
    const observer = new ResizeObserver(schedule);
    observer.observe(track);
    if (header) observer.observe(header);
    if (filters) observer.observe(filters);
    for (const nav of navs) observer.observe(nav, { box: "border-box" });
    track.addEventListener("scroll", onScroll, { passive: true });
    track.addEventListener("wheel", wheel, { passive: false });
    track.addEventListener("keydown", keyDown);
    canvas.addEventListener("click", savePosition, true);
    window.addEventListener("resize", schedule);
    schedule();
    return () => {
      cancelAnimationFrame(frame); cancelAnimationFrame(scrollFrame);
      observer.disconnect();
      track.removeEventListener("scroll", onScroll); track.removeEventListener("wheel", wheel); track.removeEventListener("keydown", keyDown);
      canvas.removeEventListener("click", savePosition, true); window.removeEventListener("resize", schedule);
      scheduleRef.current = () => {};
    };
  }, [storageKey]);

  useEffect(() => { scheduleRef.current(); }, [count]);
  useLayoutEffect(() => {
    const track = trackRef.current;
    if (!track || !layout) return;
    const saved = pendingPosition.current;
    if (saved) {
      const index = keysRef.current.indexOf(saved.key ?? "");
      track.scrollLeft = index >= 0 ? gridTilePosition(layout, index).left - saved.offset : saved.key ? 0 : saved.left;
      pendingPosition.current = undefined;
    }
    setViewport({ left: track.scrollLeft, width: track.clientWidth });
    if (track.scrollWidth <= track.clientWidth + 1) nextRef.current?.();
  }, [layout]);

  // サーバーでは最初の12件を表示し、端末のサイズが分かったら表示範囲に絞る。
  const indices = layout
    ? visibleGridIndices(layout, viewport.left, viewport.width, viewport.width / 2).filter((index) => index < count)
    : items.slice(0, 12).map((_, i) => i);
  // フォーカス中のカードは、スクロールしても外さない。
  if (Number.isInteger(focusedIndex) && focusedIndex >= 0 && focusedIndex < items.length && !indices.includes(focusedIndex)) indices.push(focusedIndex);
  const visibleKey = indices.join(",");
  useEffect(() => {
    if (layout) visibleRef.current?.(visibleKey.split(",").filter(Boolean).map(Number));
  }, [visibleKey, layout]);
  return <div ref={trackRef} onFocusCapture={(event) => setFocusedIndex(Number((event.target.closest(".home-feed-grid-tile") as HTMLElement | null)?.dataset.index))}
    onBlurCapture={(event) => { if (!event.currentTarget.contains(event.relatedTarget)) setFocusedIndex(NaN); }} role="region" aria-label="投稿一覧" tabIndex={0} className="home-feed-grid-strip focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-blue-400">
    <div className="home-feed-grid-canvas" data-count={items.length} data-ready={layout ? "true" : undefined} style={layout ? { width: layout.width, height: layout.height } : undefined}>
      {indices.sort((a, b) => a - b).map((index) => {
        const position = layout ? gridTilePosition(layout, index) : undefined;
        return <div key={keys[index]} data-key={keys[index]} data-index={index} data-position={index + 1} className="home-feed-grid-tile"
          style={position ? { ...position, width: layout?.tileWidth } : undefined}>{items[index]}</div>;
      })}
    </div>
  </div>;
}
