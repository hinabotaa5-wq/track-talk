"use client";

import Image from "next/image";
import { useEffect, useRef, useState } from "react";

type Props = {
  src: string;
  alt: string;
  wrapperClassName?: string;
  imageClassName?: string;
  sizes: string;
  priority?: boolean;
  /** 親が relative + 高さ固定のときは fill、そうでなければ width/height で描画する */
  fill?: boolean;
};

const MAX_SCALE = 4;

/** ブラウザ全体ではなく写真だけをピンチ拡大・ドラッグ移動できるようにする */
function usePinchZoom(open: boolean) {
  const overlayRef = useRef<HTMLDivElement>(null);
  const imgRef = useRef<HTMLImageElement>(null);
  const [view, setView] = useState({ scale: 1, x: 0, y: 0 });
  const viewRef = useRef(view);

  useEffect(() => {
    viewRef.current = view;
  }, [view]);

  useEffect(() => {
    const overlay = overlayRef.current;
    if (!open || !overlay) return;
    let pinch: { dist: number; scale: number } | null = null;
    let pan: { x: number; y: number; vx: number; vy: number } | null = null;

    const clamp = (scale: number, x: number, y: number) => {
      const s = Math.min(MAX_SCALE, Math.max(1, scale));
      if (s === 1) return { scale: 1, x: 0, y: 0 };
      const img = imgRef.current;
      const maxX = ((img?.clientWidth ?? 0) * (s - 1)) / 2;
      const maxY = ((img?.clientHeight ?? 0) * (s - 1)) / 2;
      return { scale: s, x: Math.min(maxX, Math.max(-maxX, x)), y: Math.min(maxY, Math.max(-maxY, y)) };
    };
    const dist = (t: TouchList) => Math.hypot(t[0].clientX - t[1].clientX, t[0].clientY - t[1].clientY);
    const startPan = (t: Touch) => {
      pan = { x: t.clientX, y: t.clientY, vx: viewRef.current.x, vy: viewRef.current.y };
    };

    const onStart = (e: TouchEvent) => {
      if (e.touches.length === 2) {
        pinch = { dist: dist(e.touches), scale: viewRef.current.scale };
        pan = null;
      } else if (e.touches.length === 1 && viewRef.current.scale > 1) {
        startPan(e.touches[0]);
      }
    };
    const onMove = (e: TouchEvent) => {
      const v = viewRef.current;
      if (pinch && e.touches.length === 2) {
        e.preventDefault();
        setView(clamp((pinch.scale * dist(e.touches)) / pinch.dist, v.x, v.y));
      } else if (pan && e.touches.length === 1) {
        e.preventDefault();
        setView(clamp(v.scale, pan.vx + e.touches[0].clientX - pan.x, pan.vy + e.touches[0].clientY - pan.y));
      }
    };
    const onEnd = (e: TouchEvent) => {
      if (e.touches.length < 2) pinch = null;
      if (e.touches.length === 0) pan = null;
      else if (e.touches.length === 1 && viewRef.current.scale > 1) startPan(e.touches[0]);
    };
    overlay.addEventListener("touchstart", onStart, { passive: true });
    overlay.addEventListener("touchmove", onMove, { passive: false });
    overlay.addEventListener("touchend", onEnd);
    overlay.addEventListener("touchcancel", onEnd);
    return () => {
      overlay.removeEventListener("touchstart", onStart);
      overlay.removeEventListener("touchmove", onMove);
      overlay.removeEventListener("touchend", onEnd);
      overlay.removeEventListener("touchcancel", onEnd);
      setView({ scale: 1, x: 0, y: 0 });
    };
  }, [open]);

  return { overlayRef, imgRef, view };
}

export function ZoomableImage({ src, alt, wrapperClassName, imageClassName, sizes, priority, fill }: Props) {
  const [open, setOpen] = useState(false);
  const { overlayRef, imgRef, view } = usePinchZoom(open);

  useEffect(() => {
    if (!open) return;
    const onKey = (e: KeyboardEvent) => {
      if (e.key === "Escape") setOpen(false);
    };
    const prevOverflow = document.body.style.overflow;
    document.body.style.overflow = "hidden";
    window.addEventListener("keydown", onKey);
    return () => {
      window.removeEventListener("keydown", onKey);
      document.body.style.overflow = prevOverflow;
    };
  }, [open]);

  return (
    <>
      <button
        type="button"
        onClick={() => setOpen(true)}
        aria-label="写真を拡大表示"
        className={`${wrapperClassName ?? ""} block cursor-zoom-in`}
      >
        {fill ? (
          <Image src={src} alt={alt} fill sizes={sizes} className={imageClassName} loading={priority ? undefined : "lazy"} priority={priority} />
        ) : (
          <Image src={src} alt={alt} width={1200} height={800} sizes={sizes} className={imageClassName} priority={priority} />
        )}
      </button>
      {open ? (
        <div
          role="dialog"
          aria-modal="true"
          aria-label="写真の拡大表示"
          ref={overlayRef}
          className="fixed inset-0 z-[100] flex touch-none items-center justify-center bg-black/90 p-4"
          onClick={() => {
            if (view.scale === 1) setOpen(false);
          }}
        >
          <button
            type="button"
            aria-label="閉じる"
            onClick={() => setOpen(false)}
            className="absolute right-4 top-4 flex h-10 w-10 items-center justify-center rounded-full bg-white/15 text-2xl text-white"
          >
            ×
          </button>
          {/* eslint-disable-next-line @next/next/no-img-element */}
          <img
            ref={imgRef}
            src={src}
            alt={alt}
            className="max-h-full max-w-full object-contain"
            style={{ transform: `translate(${view.x}px, ${view.y}px) scale(${view.scale})` }}
          />
        </div>
      ) : null}
    </>
  );
}
