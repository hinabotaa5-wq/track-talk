"use client";

import Image from "next/image";
import { useEffect, useState } from "react";

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

export function ZoomableImage({ src, alt, wrapperClassName, imageClassName, sizes, priority, fill }: Props) {
  const [open, setOpen] = useState(false);

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
          className="fixed inset-0 z-[100] flex items-center justify-center bg-black/90 p-4"
          onClick={() => setOpen(false)}
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
          <img src={src} alt={alt} className="max-h-full max-w-full object-contain" />
        </div>
      ) : null}
    </>
  );
}
