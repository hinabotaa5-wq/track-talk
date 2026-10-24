"use client";

import { Children, useEffect, useRef, type ReactNode } from "react";
import { packMosaic } from "@/lib/masonry-feed";

type FeedPosition = { left: number; href?: string; offset: number };
const positions = new Map<string, FeedPosition>();

function readNumber(key: string) {
  try {
    const value = Number(sessionStorage.getItem(key));
    return Number.isFinite(value) && value > 0 ? value : undefined;
  } catch {
    return undefined;
  }
}

function saveNumber(key: string, value: number) {
  try {
    sessionStorage.setItem(key, String(value));
  } catch {
    // プライベートブラウズなど、保存できない環境ではその表示中だけ固定する。
  }
}

export function MasonryFeed({ children, ratios, onEndReached, storageKey }: { children: ReactNode; ratios: number[]; onEndReached?: () => void; storageKey?: string }) {
  const trackRef = useRef<HTMLDivElement>(null);
  const pendingPosition = useRef<FeedPosition | undefined>(undefined);
  const items = Children.toArray(children);
  const ratioKey = ratios.join(",");

  useEffect(() => {
    const track = trackRef.current;
    const canvas = track?.firstElementChild as HTMLElement | null;
    if (!track || !canvas) return;
    pendingPosition.current ??= storageKey ? positions.get(storageKey) : undefined;
    function savePosition(event?: Event) {
      if (!storageKey || !track || !canvas || pendingPosition.current) return;
      const nodes = Array.from(canvas.children) as HTMLElement[];
      const clicked = event?.target instanceof Element ? event.target.closest("a") : null;
      const anchor = (clicked?.closest(".masonry-feed-tile") as HTMLElement | null)
        ?? nodes.find((node) => node.offsetLeft + node.offsetWidth > track.scrollLeft);
      positions.set(storageKey, {
        left: track.scrollLeft,
        href: anchor?.querySelector("a")?.getAttribute("href") ?? undefined,
        offset: (anchor?.offsetLeft ?? 0) - track.scrollLeft,
      });
    }
    function onScroll() { savePosition(); checkEnd(); }
    const hints = ratioKey.split(",").map(Number);
    let frame = 0;
    let fixedAvailable: number | undefined;
    let measuredWidth = 0;
    function checkEnd() {
      if (!track || !onEndReached) return;
      const remaining = track.scrollWidth - track.scrollLeft - track.clientWidth;
      if (remaining <= Math.max(160, track.clientWidth * 0.5)
        && (track.scrollLeft > 0 || track.scrollWidth <= track.clientWidth + 1)) onEndReached();
    }
    function layout() {
      if (!track || !canvas) return;
      const nodes = Array.from(canvas.children) as HTMLElement[];
      const bottomNav = Array.from(document.querySelectorAll<HTMLElement>('nav[aria-label="メイン"]')).find((nav) => getComputedStyle(nav).position === "fixed" && nav.getBoundingClientRect().height > 0);
      const bottom = bottomNav?.getBoundingClientRect().height ?? 0;
      const filters = document.querySelector<HTMLElement>(".home-filter-anchor");
      const header = document.querySelector("header");
      if (header) filters?.style.setProperty("--home-header-height", `${header.getBoundingClientRect().height}px`);
      // キーボードや引っ張って更新するときの viewport の一時的な変化で、投稿の大きさや順番を
      // 組み直さない。幅が変わったときだけ、表示幅に合わせて新しく計算する。
      const width = Math.round(track.clientWidth);
      if (!fixedAvailable || Math.abs(width - measuredWidth) > 1) {
        const savedAvailable = storageKey ? readNumber(`masonry-height:${storageKey}:${width}`) : undefined;
        fixedAvailable = savedAvailable ?? Math.max(280, window.innerHeight - (track.getBoundingClientRect().top + window.scrollY) - bottom - 8);
        measuredWidth = width;
        if (storageKey) saveNumber(`masonry-height:${storageKey}:${width}`, fixedAvailable);
      }
      const available = fixedAvailable;
      const anchor = track.scrollLeft > 0 ? nodes.find((node) => node.offsetLeft + node.offsetWidth > track.scrollLeft) : undefined;
      const anchorX = anchor?.offsetLeft ?? 0;
      const result = packMosaic(nodes.map((node, index) => {
        const image = node.querySelector<HTMLImageElement>("img");
        const src = image?.currentSrc || image?.src;
        const imageKey = src ? `masonry-ratio:${src}` : undefined;
        const naturalRatio = image?.naturalWidth && image.naturalHeight ? image.naturalWidth / image.naturalHeight : undefined;
        const ratio = (naturalRatio ?? (imageKey ? readNumber(imageKey) : undefined) ?? hints[index]) || 4 / 3;
        if (naturalRatio && imageKey) saveNumber(imageKey, naturalRatio);
        return { ratio, footer: 0, compact: Boolean(node.querySelector(".feed-card-text")) };
      }), track.clientWidth, available);
      canvas.style.width = `${result.width}px`;
      canvas.style.height = `${result.height}px`;
      canvas.dataset.ready = "true";
      nodes.forEach((node, index) => {
        const tile = result.tiles[index];
        node.style.left = `${tile.x}px`;
        node.style.top = `${tile.y}px`;
        node.style.width = `${tile.width}px`;
        node.dataset.position = String(index + 1);
      });
      const saved = pendingPosition.current;
      if (saved) {
        const target = nodes.find((node) => node.querySelector("a")?.getAttribute("href") === saved.href);
        track.scrollLeft = target ? target.offsetLeft - saved.offset : saved.left;
        if (target || !saved.href) pendingPosition.current = undefined;
      } else if (anchor) track.scrollLeft += anchor.offsetLeft - anchorX;
      checkEnd();
    }
    function schedule() {
      cancelAnimationFrame(frame);
      frame = requestAnimationFrame(layout);
    }
    function wheel(event: WheelEvent) {
      if (!track || !event.cancelable || event.ctrlKey || event.metaKey || event.shiftKey
        || !window.matchMedia("(hover: hover) and (pointer: fine)").matches
        || Math.abs(event.deltaX) >= Math.abs(event.deltaY)) return;
      const unit = event.deltaMode === WheelEvent.DOM_DELTA_LINE ? 16 : event.deltaMode === WheelEvent.DOM_DELTA_PAGE ? track.clientWidth : 1;
      const delta = event.deltaY * unit;
      const max = track.scrollWidth - track.clientWidth;
      if (max <= 0 || (delta > 0 && track.scrollLeft >= max - 1) || (delta < 0 && track.scrollLeft <= 1)) return;
      event.preventDefault();
      track.scrollLeft = Math.max(0, Math.min(max, track.scrollLeft + delta));
    }
    function keyDown(event: KeyboardEvent) {
      if (!track || (event.key !== "ArrowLeft" && event.key !== "ArrowRight")) return;
      event.preventDefault();
      const delta = Math.max(120, track.clientWidth * 0.72) * (event.key === "ArrowRight" ? 1 : -1);
      track.scrollLeft = Math.max(0, Math.min(track.scrollWidth - track.clientWidth, track.scrollLeft + delta));
    }
    const observer = new ResizeObserver(schedule);
    observer.observe(track);
    canvas.addEventListener("load", schedule, true);
    canvas.addEventListener("error", schedule, true);
    track.addEventListener("wheel", wheel, { passive: false });
    track.addEventListener("keydown", keyDown);
    track.addEventListener("scroll", onScroll, { passive: true });
    canvas.addEventListener("click", savePosition, true);
    window.addEventListener("resize", schedule);
    schedule();
    return () => {
      cancelAnimationFrame(frame);
      observer.disconnect();
      canvas.removeEventListener("load", schedule, true);
      canvas.removeEventListener("error", schedule, true);
      track.removeEventListener("wheel", wheel);
      track.removeEventListener("keydown", keyDown);
      track.removeEventListener("scroll", onScroll);
      canvas.removeEventListener("click", savePosition, true);
      window.removeEventListener("resize", schedule);
    };
  }, [items.length, ratioKey, onEndReached, storageKey]);

  return <div ref={trackRef} role="region" aria-label="投稿一覧" tabIndex={0} className="masonry-feed-strip rounded-xl focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-blue-400">
    <div className="masonry-feed-canvas">
      {items.map((item, index) => <div key={item && typeof item === "object" && "key" in item ? item.key : index} className="masonry-feed-tile">{item}</div>)}
    </div>
  </div>;
}
