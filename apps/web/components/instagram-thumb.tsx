"use client";

import { useState } from "react";
import { instagramMediaUrl, type InstagramType } from "@/lib/instagram";
import { PosterFallback } from "@/components/poster-card";

// Instagram の投稿・リールのカードに出す写真。Instagram が返す表紙の画像を直接読み、
// 読めなかった(非公開・削除・Instagram 側の制限)ときは、これまでどおりの文字のカードに戻す。
export function InstagramThumb({ type, shortcode, title }: { type: InstagramType; shortcode: string; title: string }) {
  const [failed, setFailed] = useState(false);
  if (failed) {
    return (
      <div className="aspect-[4/3]">
        <PosterFallback title={title} />
      </div>
    );
  }
  return (
    // eslint-disable-next-line @next/next/no-img-element
    <img
      src={instagramMediaUrl(type, shortcode)}
      alt=""
      loading="lazy"
      referrerPolicy="no-referrer"
      onError={() => setFailed(true)}
      className="block aspect-[4/5] w-full bg-black/25 object-cover"
    />
  );
}
