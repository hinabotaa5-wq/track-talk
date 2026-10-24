"use client";

import { Children, useEffect, useRef, type ReactNode } from "react";
import { packMosaic } from "@/lib/masonry-feed";

type FeedPosition = { left: number; href?: string; offset: number };
const positions = new Map<string, FeedPosition>();

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
      const viewHeight = window.visualViewport?.height ?? window.innerHeight;
      // 上下のパディング8pxだけを残し、下のメニュー直前まで使う。
      const available = Math.max(280, viewHeight - (track.getBoundingClientRect().top + window.scrollY) - bottom - 8);
      const anchor = track.scrollLeft > 0 ? nodes.find((node) => node.offsetLeft + node.offsetWidth > track.scrollLeft) : undefined;
      const anchorX = anchor?.offsetLeft ?? 0;
      const result = packMosaic(nodes.map((node, index) => {
        const image = node.querySelector<HTMLImageElement>("img");
        const ratio = image?.naturalWidth && image.naturalHeight ? image.naturalWidth / image.naturalHeight : hints[index] || 4 / 3;
        return { ratio, footer: 0 };
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
    const observer = new ResizeObserver(schedule);
    observer.observe(track);
    canvas.addEventListener("load", schedule, true);
    canvas.addEventListener("error", schedule, true);
    track.addEventListener("wheel", wheel, { passive: false });
    track.addEventListener("scroll", onScroll, { passive: true });
    canvas.addEventListener("click", savePosition, true);
    window.addEventListener("resize", schedule);
    window.visualViewport?.addEventListener("resize", schedule);
    schedule();
    return () => {
      cancelAnimationFrame(frame);
      observer.disconnect();
      canvas.removeEventListener("load", schedule, true);
      canvas.removeEventListener("error", schedule, true);
      track.removeEventListener("wheel", wheel);
      track.removeEventListener("scroll", onScroll);
      canvas.removeEventListener("click", savePosition, true);
      window.removeEventListener("resize", schedule);
      window.visualViewport?.removeEventListener("resize", schedule);
    };
  }, [items.length, ratioKey, onEndReached, storageKey]);

  return <div ref={trackRef} role="region" aria-label="投稿一覧" tabIndex={0} className="masonry-feed-strip rounded-xl focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-blue-400">
    <div className="masonry-feed-canvas">
      {items.map((item, index) => <div key={item && typeof item === "object" && "key" in item ? item.key : index} className="masonry-feed-tile">{item}</div>)}
    </div>
  </div>;
}
