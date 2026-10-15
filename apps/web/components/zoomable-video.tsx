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
  const videoRef = useRef<HTMLVideoElement>(null);
  const [playing, setPlaying] = useState(false);
  const [showControls, setShowControls] = useState(false);
  const zoomed = view.scale > 1;

  useEffect(() => {
    viewRef.current = view;
  }, [view]);

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

  const togglePlay = () => {
    const v = videoRef.current;
    if (!v) return;
    if (v.paused) void v.play();
    else v.pause();
  };
  const skip = (sec: number) => {
    const v = videoRef.current;
    if (!v) return;
    v.currentTime = Math.min(v.duration || 0, Math.max(0, v.currentTime + sec));
  };

  return (
    <div ref={wrapRef} className="relative overflow-hidden rounded-xl bg-black">
      <video
        ref={videoRef}
        className={className}
        style={{
          transform: `translate(${view.x}px, ${view.y}px) scale(${view.scale})`,
          transformOrigin: "center",
          touchAction: "pan-y",
        }}
        src={src}
        poster={poster ?? undefined}
        controls={!zoomed}
        onPlay={() => setPlaying(true)}
        onPause={() => setPlaying(false)}
        onClick={zoomed ? () => setShowControls((v) => !v) : undefined}
        playsInline
        preload="metadata"
      />
      {zoomed && showControls ? (
        <div className="pointer-events-none absolute inset-0 flex items-center justify-center gap-6">
          <button
            type="button"
            onClick={() => skip(-10)}
            className="pointer-events-auto rounded-full bg-black/60 px-3 py-2 text-sm text-white"
            aria-label="10秒戻る"
          >
            -10
          </button>
          <button
            type="button"
            onClick={togglePlay}
            className="pointer-events-auto rounded-full bg-black/60 px-4 py-2 text-sm text-white"
            aria-label={playing ? "一時停止" : "再生"}
          >
            {playing ? "❚❚" : "▶"}
          </button>
          <button
            type="button"
            onClick={() => skip(10)}
            className="pointer-events-auto rounded-full bg-black/60 px-3 py-2 text-sm text-white"
            aria-label="10秒進む"
          >
            +10
          </button>
        </div>
      ) : null}
      {zoomed ? (
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
