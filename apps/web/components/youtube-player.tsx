"use client";

import { useEffect, useRef, useState } from "react";
import Image from "next/image";
import { Play } from "lucide-react";

const youtubeOrigin = "https://www.youtube-nocookie.com";
const playRetryMs = 600;
const playRetryLimit = 8;

// ポスターを押した1回で再生を始める。autoplay が効かないと YouTube 自身の再生ボタンが出て、2回押すことになるので、
// プレーヤーの準備ができたという知らせ(onReady)を待って再生を頼み、再生が始まるまで数回頼み直す。
function YouTubeFrame({ videoId, title }: { videoId: string; title: string }) {
  const frameRef = useRef<HTMLIFrameElement>(null);

  useEffect(() => {
    const frame = frameRef.current;
    if (!frame) return;
    let playing = false;
    let tries = 0;
    function post(message: object) {
      frame?.contentWindow?.postMessage(JSON.stringify(message), youtubeOrigin);
    }
    function play() {
      post({ event: "command", func: "playVideo", args: [] });
    }
    function onMessage(e: MessageEvent) {
      if (e.origin !== youtubeOrigin || e.source !== frame?.contentWindow) return;
      let data: { event?: string; info?: { playerState?: number } } | null = null;
      try {
        data = typeof e.data === "string" ? JSON.parse(e.data) : e.data;
      } catch {
        return;
      }
      if (data?.event === "onReady" || data?.event === "initialDelivery") play();
      const state = data?.info?.playerState;
      if (data?.event === "infoDelivery" && (state === 1 || state === 3)) playing = true;
    }
    window.addEventListener("message", onMessage);
    // 準備の知らせは、こちらから「聞いています」と伝えると届く。iframe の読み込みが終わってから伝える。
    function listen() {
      post({ event: "listening", id: 1, channel: "widget" });
      play();
    }
    frame.addEventListener("load", listen);
    const timer = window.setInterval(() => {
      if (playing || tries >= playRetryLimit) {
        window.clearInterval(timer);
        return;
      }
      tries++;
      play();
    }, playRetryMs);
    return () => {
      window.removeEventListener("message", onMessage);
      frame.removeEventListener("load", listen);
      window.clearInterval(timer);
    };
  }, [videoId]);

  return (
    <iframe
      ref={frameRef}
      className="aspect-video w-full rounded-xl shadow-sm"
      src={`${youtubeOrigin}/embed/${videoId}?autoplay=1&playsinline=1&enablejsapi=1`}
      title={title}
      allow="accelerometer; autoplay; clipboard-write; encrypted-media; gyroscope; picture-in-picture; web-share"
      allowFullScreen
      referrerPolicy="strict-origin-when-cross-origin"
    />
  );
}

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
    return <YouTubeFrame videoId={videoId} title={title} />;
  }

  return (
    <button
      type="button"
      onClick={() => setIsPlaying(true)}
      aria-label={`YouTube動画「${title}」を再生`}
      className="group relative aspect-video w-full overflow-hidden rounded-xl bg-black text-left shadow-sm transition focus:outline-none focus-visible:ring-2 focus-visible:ring-blue-500 active:scale-95"
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
