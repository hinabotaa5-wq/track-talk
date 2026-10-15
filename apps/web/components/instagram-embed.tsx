"use client";

import { useEffect, useRef, useState } from "react";
import { ExternalLink } from "lucide-react";
import { instagramPermalink, isInstagramShortcode, type InstagramType } from "@/lib/instagram";

const instagramOrigin = "https://www.instagram.com";

// Instagram の埋め込みは、読み込めると親ページへ高さ(MEASURE)を知らせてくる。
// 届かないまま時間が経ったら、非公開・削除・通信の問題のどれかなので、リンクで開けるように案内する。
const slowAfterMs = 8000;

export function InstagramEmbed({ type, shortcode, title }: { type: string; shortcode: string; title: string }) {
  const frameRef = useRef<HTMLIFrameElement>(null);
  const [height, setHeight] = useState<number | null>(null);
  const [slow, setSlow] = useState(false);
  const valid = (type === "p" || type === "reel") && isInstagramShortcode(shortcode);

  useEffect(() => {
    if (!valid) return;
    function onMessage(e: MessageEvent) {
      if (e.origin !== instagramOrigin || e.source !== frameRef.current?.contentWindow) return;
      try {
        const data = typeof e.data === "string" ? JSON.parse(e.data) : e.data;
        const next = Number(data?.details?.height);
        if (data?.type === "MEASURE" && Number.isFinite(next) && next > 0) setHeight(Math.min(Math.round(next), 2000));
      } catch {
        // Instagram 以外の形のメッセージは無視する
      }
    }
    window.addEventListener("message", onMessage);
    const timer = window.setTimeout(() => setSlow(true), slowAfterMs);
    return () => {
      window.removeEventListener("message", onMessage);
      window.clearTimeout(timer);
    };
  }, [valid]);

  if (!valid) return null;
  const permalink = instagramPermalink(type as InstagramType, shortcode);
  const link = (
    <a
      href={permalink}
      target="_blank"
      rel="noopener noreferrer"
      className="inline-flex items-center gap-1.5 text-sm font-medium text-blue-300 underline-offset-2 hover:underline"
    >
      Instagramで見る
      <ExternalLink className="size-3.5" aria-hidden />
    </a>
  );

  return (
    <div className="mx-auto w-full max-w-[540px] space-y-2">
      <iframe
        ref={frameRef}
        src={`${permalink}embed/`}
        title={`Instagram「${title}」`}
        className="w-full overflow-hidden rounded-xl border border-white/20 bg-white"
        style={{ height: height ?? (type === "reel" ? 720 : 600) }}
        loading="lazy"
        scrolling="no"
        allow="encrypted-media; picture-in-picture; web-share"
        sandbox="allow-scripts allow-same-origin allow-popups allow-popups-to-escape-sandbox"
        referrerPolicy="strict-origin-when-cross-origin"
      />
      {slow && height === null ? (
        <p className="text-sm text-slate-200">
          表示されないときは、非公開か削除された投稿かもしれません。Instagramで開いて確かめてください。
        </p>
      ) : null}
      {link}
    </div>
  );
}
