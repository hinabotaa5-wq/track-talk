"use client";

import { bindExpandedPlayback, hasExpandedPlayback } from "@/lib/expanded-playback";
import { useEffect, useRef, useState } from "react";
import { YouTubePlayer } from "./youtube-player";

export function PlaybackSlot({
  postId,
  kind,
  title,
  playbackUrl,
  youtubeId,
  thumbnailUrl,
}: {
  postId: string;
  kind: "video" | "youtube";
  title: string;
  playbackUrl?: string;
  youtubeId?: string;
  thumbnailUrl?: string;
}) {
  const slotRef = useRef<HTMLDivElement>(null);
  const [fromThumbnail] = useState(() => hasExpandedPlayback(postId));

  useEffect(() => {
    const slot = slotRef.current;
    if (!fromThumbnail || !slot) return;
    return bindExpandedPlayback(postId, slot);
  }, [fromThumbnail, postId]);

  if (fromThumbnail) {
    return <div ref={slotRef} className="aspect-video w-full overflow-hidden rounded-xl bg-black" />;
  }
  if (kind === "video" && playbackUrl) {
    return (
      <video
        className="aspect-video w-full rounded-xl bg-black"
        src={playbackUrl}
        poster={thumbnailUrl}
        controls
        playsInline
        autoPlay
        preload="metadata"
      />
    );
  }
  if (kind === "youtube" && youtubeId) {
    return <YouTubePlayer videoId={youtubeId} title={title} thumbnailUrl={thumbnailUrl} startPlaying />;
  }
  return null;
}
