"use client";

import { ChevronLeft, ChevronRight } from "lucide-react";
import { Children, useEffect, useId, useRef, useState, type ReactNode } from "react";

export function HorizontalFeed({ children, label = "投稿一覧", itemLabel = "投稿" }: { children: ReactNode; label?: string; itemLabel?: string }) {
  const trackRef = useRef<HTMLDivElement>(null);
  const trackID = useId();
  const hintID = useId();
  const items = Children.toArray(children);
  const [edges, setEdges] = useState({ previous: false, next: false });

  useEffect(() => {
    const track = trackRef.current;
    if (!track) return;
    function updateEdges() {
      if (!track) return;
      setEdges({ previous: track.scrollLeft > 2, next: track.scrollLeft + track.clientWidth < track.scrollWidth - 2 });
    }
    // PCの縦ホイールを横移動へ変換する。横ジェスチャーと拡大操作はブラウザに任せる。
    function onWheel(event: WheelEvent) {
      if (!track || !event.cancelable || event.ctrlKey || event.metaKey || event.shiftKey) return;
      if (!window.matchMedia("(hover: hover) and (pointer: fine)").matches) return;
      if (Math.abs(event.deltaX) >= Math.abs(event.deltaY)) return;
      const unit = event.deltaMode === WheelEvent.DOM_DELTA_LINE
        ? Number.parseFloat(getComputedStyle(track).lineHeight) || 16
        : event.deltaMode === WheelEvent.DOM_DELTA_PAGE ? track.clientWidth : 1;
      const delta = -event.deltaY * unit;
      const max = track.scrollWidth - track.clientWidth;
      if (max <= 0 || (delta > 0 && track.scrollLeft >= max - 1) || (delta < 0 && track.scrollLeft <= 1)) return;
      event.preventDefault();
      track.scrollLeft = Math.max(0, Math.min(max, track.scrollLeft + delta));
    }
    const frame = requestAnimationFrame(updateEdges);
    const observer = new ResizeObserver(updateEdges);
    observer.observe(track);
    track.addEventListener("scroll", updateEdges, { passive: true });
    track.addEventListener("wheel", onWheel, { passive: false });
    return () => {
      cancelAnimationFrame(frame);
      observer.disconnect();
      track.removeEventListener("scroll", updateEdges);
      track.removeEventListener("wheel", onWheel);
    };
  }, [items.length]);

  function move(direction: number) {
    const track = trackRef.current;
    const first = track?.firstElementChild;
    if (!track || !first) return;
    const gap = Number.parseFloat(getComputedStyle(track).columnGap) || 0;
    track.scrollBy({
      left: direction * (first.getBoundingClientRect().width + gap),
      behavior: window.matchMedia("(prefers-reduced-motion: reduce)").matches ? "instant" : "smooth",
    });
  }

  const buttonClass = "inline-flex size-9 items-center justify-center rounded-full border border-white/20 bg-white/10 text-slate-200 transition hover:bg-white/16 active:scale-[0.98] disabled:opacity-30";
  return (
    <section className="min-w-0 space-y-1">
      <div className="flex items-center justify-between gap-2 px-1">
        <p id={hintID} className="text-xs text-slate-300">
          <span className="horizontal-feed-touch-hint">左にスワイプして次の{itemLabel}へ</span>
          <span className="horizontal-feed-wheel-hint">ホイールで左右にスクロール</span>
        </p>
        <div className="flex shrink-0 gap-1">
          <button type="button" aria-label={`前の${itemLabel}へ`} aria-controls={trackID} disabled={!edges.previous} onClick={() => move(-1)} className={buttonClass}>
            <ChevronLeft className="size-4" aria-hidden />
          </button>
          <button type="button" aria-label={`次の${itemLabel}へ`} aria-controls={trackID} disabled={!edges.next} onClick={() => move(1)} className={buttonClass}>
            <ChevronRight className="size-4" aria-hidden />
          </button>
        </div>
      </div>
      <div ref={trackRef} id={trackID} role="region" aria-label={label} aria-describedby={hintID} tabIndex={0} className="horizontal-feed-track rounded-xl focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-blue-400">
        {items.map((item, index) => <div key={item && typeof item === "object" && "key" in item ? item.key : index} className="horizontal-feed-slide">{item}</div>)}
      </div>
    </section>
  );
}
