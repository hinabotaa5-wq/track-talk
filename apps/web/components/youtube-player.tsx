"use client";

import { useState } from "react";
import Image from "next/image";
import { Play } from "lucide-react";

export function YouTubePlayer({
  videoId,
  title,
  thumbnailUrl,
}: {
  videoId: string;
  title: string;
  thumbnailUrl?: string;
}) {
  const [isPlaying, setIsPlaying] = useState(false);
  const poster = thumbnailUrl || `https://i.ytimg.com/vi/${videoId}/hqdefault.jpg`;

  if (isPlaying) {
    return (
      <iframe
        className="aspect-video w-full rounded-xl shadow-sm"
        src={`https://www.youtube-nocookie.com/embed/${videoId}?autoplay=1`}
        title={title}
        allow="accelerometer; autoplay; clipboard-write; encrypted-media; gyroscope; picture-in-picture; web-share"
        allowFullScreen
        loading="lazy"
      />
    );
  }

  return (
    <button
      type="button"
      onClick={() => setIsPlaying(true)}
      aria-label={`YouTube動画「${title}」を再生`}
      className="group relative aspect-video w-full overflow-hidden rounded-xl bg-black text-left shadow-sm focus:outline-none focus-visible:ring-2 focus-visible:ring-blue-500"
    >
      <Image
        src={poster}
        alt={title}
        fill
        sizes="(max-width: 1024px) 100vw, 768px"
        className="object-cover transition duration-300 group-hover:scale-105 group-hover:opacity-90"
        loading="lazy"
      />
      <div className="absolute inset-0 bg-black/20 transition group-hover:bg-black/30" />
      <div className="absolute inset-0 flex items-center justify-center">
        <div className="flex h-14 w-14 items-center justify-center rounded-full bg-red-600 text-white shadow-lg transition duration-200 group-hover:scale-110 group-hover:bg-red-700">
          <Play className="ml-0.5 h-6 w-6 fill-current" />
        </div>
      </div>
    </button>
  );
}
