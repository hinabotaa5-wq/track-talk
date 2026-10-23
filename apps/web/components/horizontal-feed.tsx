"use client";

import { Children, useEffect, useRef, type ReactNode } from "react";

export function HorizontalFeed({ children, label = "投稿一覧" }: { children: ReactNode; label?: string }) {
  const trackRef = useRef<HTMLDivElement>(null);
  const items = Children.toArray(children);

  useEffect(() => {
    const track = trackRef.current;
    if (!track) return;
    // PCの縦ホイールを横移動へ変換する。横ジェスチャーと拡大操作はブラウザに任せる。
    function onWheel(event: WheelEvent) {
      if (!track || !event.cancelable || event.ctrlKey || event.metaKey || event.shiftKey) return;
      if (!window.matchMedia("(hover: hover) and (pointer: fine)").matches) return;
      if (Math.abs(event.deltaX) >= Math.abs(event.deltaY)) return;
      const unit = event.deltaMode === WheelEvent.DOM_DELTA_LINE
        ? Number.parseFloat(getComputedStyle(track).lineHeight) || 16
        : event.deltaMode === WheelEvent.DOM_DELTA_PAGE ? track.clientWidth : 1;
      const delta = event.deltaY * unit;
      const max = track.scrollWidth - track.clientWidth;
      if (max <= 0 || (delta > 0 && track.scrollLeft >= max - 1) || (delta < 0 && track.scrollLeft <= 1)) return;
      event.preventDefault();
      track.scrollLeft = Math.max(0, Math.min(max, track.scrollLeft + delta));
    }
    track.addEventListener("wheel", onWheel, { passive: false });
    return () => {
      track.removeEventListener("wheel", onWheel);
    };
  }, [items.length]);

  return (
    <section className="min-w-0">
      <div ref={trackRef} role="region" aria-label={label} tabIndex={0} className="horizontal-feed-track rounded-xl focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-blue-400">
        {items.map((item, index) => <div key={item && typeof item === "object" && "key" in item ? item.key : index} className="horizontal-feed-slide">{item}</div>)}
      </div>
    </section>
  );
}
