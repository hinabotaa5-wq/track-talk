"use client";

import { useEffect, useRef, useState } from "react";

type Props = {
  src: string;
  poster?: string | null;
  className?: string;
};

const MAX_SCALE = 4;

export function ZoomableVideo({ src, poster, className }: Props) {
  const wrapRef = useRef<HTMLDivElement>(null);
  const [view, setView] = useState({ scale: 1, x: 0, y: 0 });
  const viewRef = useRef(view);
  viewRef.current = view;

  useEffect(() => {
    const el = wrapRef.current;
    if (!el) return;

    let pinch: { dist: number; scale: number } | null = null;
    let pan: { x: number; y: number; vx: number; vy: number } | null = null;

    const clamp = (scale: number, x: number, y: number) => {
      const s = Math.min(MAX_SCALE, Math.max(1, scale));
      if (s === 1) return { scale: 1, x: 0, y: 0 };
      const maxX = (el.clientWidth * (s - 1)) / 2;
      const maxY = (el.clientHeight * (s - 1)) / 2;
      return { scale: s, x: Math.min(maxX, Math.max(-maxX, x)), y: Math.min(maxY, Math.max(-maxY, y)) };
    };
    const dist = (t: TouchList) => Math.hypot(t[0].clientX - t[1].clientX, t[0].clientY - t[1].clientY);

    const onStart = (e: TouchEvent) => {
      if (e.touches.length === 2) {
        pinch = { dist: dist(e.touches), scale: viewRef.current.scale };
        pan = null;
      } else if (e.touches.length === 1 && viewRef.current.scale > 1) {
        pan = { x: e.touches[0].clientX, y: e.touches[0].clientY, vx: viewRef.current.x, vy: viewRef.current.y };
      }
    };
    const onMove = (e: TouchEvent) => {
      if (pinch && e.touches.length === 2) {
        e.preventDefault();
        const v = viewRef.current;
        setView(clamp((pinch.scale * dist(e.touches)) / pinch.dist, v.x, v.y));
      } else if (pan && e.touches.length === 1) {
        e.preventDefault();
        const v = viewRef.current;
        setView(clamp(v.scale, pan.vx + e.touches[0].clientX - pan.x, pan.vy + e.touches[0].clientY - pan.y));
      }
    };
    const onEnd = (e: TouchEvent) => {
      if (e.touches.length < 2) pinch = null;
      if (e.touches.length === 0) pan = null;
      else if (e.touches.length === 1 && viewRef.current.scale > 1) {
        pan = { x: e.touches[0].clientX, y: e.touches[0].clientY, vx: viewRef.current.x, vy: viewRef.current.y };
      }
    };

    el.addEventListener("touchstart", onStart, { passive: true });
    el.addEventListener("touchmove", onMove, { passive: false });
    el.addEventListener("touchend", onEnd);
    el.addEventListener("touchcancel", onEnd);
    return () => {
      el.removeEventListener("touchstart", onStart);
      el.removeEventListener("touchmove", onMove);
      el.removeEventListener("touchend", onEnd);
      el.removeEventListener("touchcancel", onEnd);
    };
  }, []);

  return (
    <div ref={wrapRef} className="relative overflow-hidden rounded-xl bg-black">
      <video
        className={className}
        style={{
          transform: `translate(${view.x}px, ${view.y}px) scale(${view.scale})`,
          transformOrigin: "center",
          touchAction: "pan-y",
        }}
        src={src}
        poster={poster ?? undefined}
        controls
        playsInline
        preload="metadata"
      />
      {view.scale > 1 ? (
        <button
          type="button"
          onClick={() => setView({ scale: 1, x: 0, y: 0 })}
          className="absolute right-2 top-2 rounded-full bg-black/60 px-3 py-1 text-xs text-white"
        >
          元に戻す
        </button>
      ) : null}
    </div>
  );
}
