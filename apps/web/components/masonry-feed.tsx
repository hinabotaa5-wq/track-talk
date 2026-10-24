"use client";

import { Children, useEffect, useLayoutEffect, useMemo, useRef, useState, type ReactNode } from "react";
import { createMosaicPacker, visibleMosaicIndices, type MosaicLayout } from "@/lib/masonry-feed";
import { BoundedCache } from "@/lib/bounded-cache";

type FeedPosition = { left: number; key?: string; offset: number };
const positions = new BoundedCache<FeedPosition>(20, 30 * 60_000);
const heights = new BoundedCache<number>(40, 30 * 60_000);
const measuredRatios = new BoundedCache<number>(6000, 30 * 60_000);
const imageRatios = new BoundedCache<number>(512, 24 * 60 * 60_000);

// 画像比率は一つの保存領域にまとめ、画像ごとにsessionStorageの項目を増やさない。
let ratiosLoaded = false;
function readRatio(key: string) {
  if (!ratiosLoaded && typeof window !== "undefined") {
    ratiosLoaded = true;
    try {
      const saved = JSON.parse(sessionStorage.getItem("masonry-ratios-v2") ?? "null");
      if (saved && Date.now() - saved.at < 24 * 60 * 60_000 && Array.isArray(saved.entries)) {
        for (const [src, ratio] of saved.entries.slice(-512)) {
          if (typeof src === "string" && typeof ratio === "number" && ratio > 0 && Number.isFinite(ratio)) imageRatios.set(src, ratio);
        }
      }
    } catch { /* 保存できない環境でも現在の一覧を表示する。 */ }
  }
  return imageRatios.get(key);
}
function persistRatios() {
  try {
    const entries = [...imageRatios.keys()].map((src) => [src, imageRatios.get(src)]);
    sessionStorage.setItem("masonry-ratios-v2", JSON.stringify({ at: Date.now(), entries }));
  } catch { /* メモリ上の比率を使う。 */ }
}

export function MasonryFeed({ children, ratios, compact = [], onEndReached, onVisibleIndices, storageKey }: {
  children: ReactNode; ratios: number[]; compact?: boolean[]; onEndReached?: () => void; onVisibleIndices?: (indices: number[]) => void; storageKey?: string;
}) {
  const trackRef = useRef<HTMLDivElement>(null);
  const items = Children.toArray(children);
  const keys = items.map((item, i) => item && typeof item === "object" && "key" in item ? String(item.key) : String(i));
  const descriptors = useRef({ ratios, compact, keys });
  const pack = useMemo(() => createMosaicPacker(), []);
  const [layout, setLayout] = useState<MosaicLayout | null>(null);
  const layoutRef = useRef(layout);
  const [viewport, setViewport] = useState({ left: 0, width: 0 });
  const pendingPosition = useRef<FeedPosition | undefined>(undefined);
  const visibleRef = useRef(onVisibleIndices);
  const nextRef = useRef(onEndReached);
  const [focusedIndex, setFocusedIndex] = useState<number>(NaN);
  useLayoutEffect(() => {
    descriptors.current = { ratios, compact, keys };
    layoutRef.current = layout;
    visibleRef.current = onVisibleIndices;
    nextRef.current = onEndReached;
  });
  const scheduleRef = useRef<() => void>(() => {});
  const signature = keys.join("|") + ":" + ratios.join(",") + ":" + compact.join(",");

  useEffect(() => {
    const track = trackRef.current;
    if (!track) return;
    pendingPosition.current = storageKey ? positions.get(storageKey) : undefined;
    let frame = 0, scrollFrame = 0, persistTimer: ReturnType<typeof setTimeout> | undefined;
    let fixedAvailable = 0, measuredWidth = 0;
    const canvas = track.firstElementChild as HTMLElement;
    function savePosition(event?: Event) {
      if (!storageKey || pendingPosition.current) return;
      const nodes = Array.from(canvas.children) as HTMLElement[];
      const clicked = event?.target instanceof Element ? event.target.closest(".masonry-feed-tile") as HTMLElement | null : null;
      const anchor = clicked ?? nodes.find((node) => node.offsetLeft + node.offsetWidth > track!.scrollLeft);
      positions.set(storageKey, { left: track!.scrollLeft, key: anchor?.dataset.key, offset: (anchor?.offsetLeft ?? 0) - track!.scrollLeft });
    }
    function layoutNow() {
      const width = Math.round(track!.clientWidth);
      if (width <= 0) return;
      if (!fixedAvailable || Math.abs(width - measuredWidth) > 1) {
        const bottomNav = Array.from(document.querySelectorAll<HTMLElement>('nav[aria-label="メイン"]')).find((nav) => getComputedStyle(nav).position === "fixed" && nav.getBoundingClientRect().height > 0);
        const filters = document.querySelector<HTMLElement>(".home-filter-anchor");
        const header = document.querySelector("header");
        if (header) filters?.style.setProperty("--home-header-height", `${header.getBoundingClientRect().height}px`);
        const heightKey = `${storageKey}:${width}`;
        fixedAvailable = heights.get(heightKey) ?? Math.max(280, window.innerHeight - (track!.getBoundingClientRect().top + window.scrollY) - (bottomNav?.getBoundingClientRect().height ?? 0) - 8);
        heights.set(heightKey, fixedAvailable);
        measuredWidth = width;
      }
      // 描画中の画像だけ測る。画面外の座標はDOMに依存しない。
      for (const node of Array.from(canvas.children) as HTMLElement[]) {
        const image = node.querySelector<HTMLImageElement>("img");
        const src = image?.currentSrc || image?.src;
        const ratio = image?.naturalWidth && image.naturalHeight ? image.naturalWidth / image.naturalHeight : undefined;
        const known = src ? readRatio(src) : undefined;
        if (node.dataset.key && (ratio || known)) measuredRatios.set(`${storageKey}:${node.dataset.key}`, (ratio || known)!);
        if (ratio && src && ratio !== known) {
          imageRatios.set(src, ratio);
          clearTimeout(persistTimer);
          persistTimer = setTimeout(persistRatios, 200);
        }
      }
      const { ratios: hints, compact: small, keys: ids } = descriptors.current;
      const result = pack(ids.map((id, i) => ({ ratio: measuredRatios.get(`${storageKey}:${id}`) ?? hints[i] ?? 4 / 3, footer: 0, compact: small[i] })), width, fixedAvailable);
      // 計算後のReact描画で横幅を反映してから、保存位置を復元する。
      if (!pendingPosition.current && layoutRef.current && track!.scrollLeft > 0) {
        const old = layoutRef.current;
        const index = visibleMosaicIndices(old, track!.scrollLeft, 1, 0)[0];
        if (index !== undefined) pendingPosition.current = { left: track!.scrollLeft, key: ids[index], offset: old.tiles[index].x - track!.scrollLeft };
      }
      setLayout(result);
    }
    function schedule() { cancelAnimationFrame(frame); frame = requestAnimationFrame(layoutNow); }
    scheduleRef.current = schedule;
    function onScroll() {
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
    canvas.addEventListener("load", schedule, true);
    canvas.addEventListener("error", schedule, true);
    track.addEventListener("scroll", onScroll, { passive: true });
    track.addEventListener("wheel", wheel, { passive: false });
    track.addEventListener("keydown", keyDown);
    canvas.addEventListener("click", savePosition, true);
    window.addEventListener("resize", schedule);
    schedule();
    return () => {
      cancelAnimationFrame(frame); cancelAnimationFrame(scrollFrame); clearTimeout(persistTimer);
      persistRatios(); observer.disconnect();
      canvas.removeEventListener("load", schedule, true); canvas.removeEventListener("error", schedule, true);
      track.removeEventListener("scroll", onScroll); track.removeEventListener("wheel", wheel); track.removeEventListener("keydown", keyDown);
      canvas.removeEventListener("click", savePosition, true); window.removeEventListener("resize", schedule);
      scheduleRef.current = () => {};
    };
  }, [pack, storageKey]);

  useEffect(() => { scheduleRef.current(); }, [signature]);
  useLayoutEffect(() => {
    const track = trackRef.current;
    if (!track || !layout) return;
    const saved = pendingPosition.current;
    if (saved) {
      const index = descriptors.current.keys.indexOf(saved.key ?? "");
      track.scrollLeft = index >= 0 ? layout.tiles[index].x - saved.offset : saved.key ? 0 : saved.left;
      pendingPosition.current = undefined;
    }
    setViewport({ left: track.scrollLeft, width: track.clientWidth });
    if (track.scrollWidth <= track.clientWidth + 1) nextRef.current?.();
  }, [layout]);

  // サーバーでは最初の12件を表示し、端末のサイズが分かったら表示範囲に絞る。
  const indices = layout ? visibleMosaicIndices(layout, viewport.left, viewport.width || 390) : items.slice(0, 12).map((_, i) => i);
  // フォーカス中のカードは、スクロールしても外さない。
  if (Number.isInteger(focusedIndex) && focusedIndex >= 0 && focusedIndex < items.length && !indices.includes(focusedIndex)) indices.push(focusedIndex);
  const visibleKey = indices.join(",");
  useEffect(() => {
    if (layout) visibleRef.current?.(visibleKey.split(",").filter(Boolean).map(Number));
  }, [visibleKey, layout]);
  return <div ref={trackRef} onFocusCapture={(event) => setFocusedIndex(Number((event.target.closest(".masonry-feed-tile") as HTMLElement | null)?.dataset.index))}
    onBlurCapture={(event) => { if (!event.currentTarget.contains(event.relatedTarget)) setFocusedIndex(NaN); }} role="region" aria-label="投稿一覧" tabIndex={0} className="masonry-feed-strip rounded-xl focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-blue-400">
    <div className="masonry-feed-canvas" data-count={items.length} data-ready={layout ? "true" : undefined} style={layout ? { width: layout.width, height: layout.height } : undefined}>
      {indices.sort((a, b) => a - b).map((index) => {
        const tile = layout?.tiles[index];
        return <div key={keys[index]} data-key={keys[index]} data-index={index} data-position={index + 1} className="masonry-feed-tile"
          style={tile ? { left: tile.x, top: tile.y, width: tile.width } : undefined}>{items[index]}</div>;
      })}
    </div>
  </div>;
}
