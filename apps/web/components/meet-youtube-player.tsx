"use client";
import { useEffect, useRef, useState } from "react";
import { loadYouTubeAPI, type VolumePlayer } from "@/lib/youtube-api";

export function MeetYouTubePlayer({ videoId }: { videoId: string }) {
  const frame = useRef<HTMLIFrameElement>(null);
  const player = useRef<VolumePlayer | null>(null);
  const desired = useRef(100);
  const starting = useRef(false);
  const [volume, setVolume] = useState(100);
  const [error, setError] = useState("");
  const src = `https://www.youtube-nocookie.com/embed/${videoId}?playsinline=1&rel=0&cc_load_policy=0&enablejsapi=1`;
  useEffect(() => {
    if (frame.current) { const url = new URL(frame.current.src); url.searchParams.set("origin", window.location.origin); frame.current.src = url.toString(); }
    return () => { player.current?.destroy(); player.current = null; };
  }, [videoId]);
  async function change(value: number) {
    desired.current = value; setVolume(value); setError("");
    const apply = (p: VolumePlayer) => { p.setVolume(desired.current); if (desired.current === 0) p.mute(); else p.unMute(); if (Math.abs(p.getVolume() - desired.current) > 1) setError("この端末では動画の音量を本体またはプレーヤーで調整してください。"); };
    if (player.current) { apply(player.current); return; }
    if (starting.current) return;
    starting.current = true;
    try {
      const api = await loadYouTubeAPI();
      if (!frame.current?.isConnected) { starting.current = false; return; }
      new api.Player(frame.current, { events: { onReady: ({ target }) => { starting.current = false; if (!frame.current?.isConnected) { target.destroy(); return; } player.current = target; apply(target); } } });
    } catch { starting.current = false; setError("音量を調整できません。動画プレーヤーの音量ボタンをご利用ください。"); }
  }
  return <div className="space-y-2"><iframe ref={frame} className="mx-auto aspect-video w-full max-w-[560px] rounded-xl bg-black shadow-sm" src={src} title="大会のYouTube生配信" loading="lazy" allow="accelerometer; autoplay; clipboard-write; encrypted-media; gyroscope; picture-in-picture; web-share" allowFullScreen referrerPolicy="strict-origin-when-cross-origin" /><label className="flex items-center gap-3 text-sm">動画の音量<input type="range" min={0} max={100} value={volume} aria-label="動画の音量" className="min-w-0 flex-1" onChange={(e) => void change(Number(e.target.value))} /><span className="w-9 text-right text-xs">{volume}%</span></label>{error ? <p role="status" className="text-xs text-slate-300">{error}</p> : null}</div>;
}
