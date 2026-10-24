"use client";

import { useEffect, useState } from "react";
import { apiFetch } from "@/lib/api";
import { meetExternalURL, splitMeetBody, type MeetResources } from "@/lib/meet-resources";
import { YouTubePlayer } from "@/components/youtube-player";

const linkClass = "w-fit break-all text-base text-blue-300 underline underline-offset-4 hover:text-blue-200";

export function MeetRoomContent({ postId, body }: { postId: string; body: string }) {
  const [resources, setResources] = useState<MeetResources | null>(null);
  useEffect(() => {
    let active = true;
    apiFetch<MeetResources>(`/v1/posts/${postId}/meet-resources`).then((data) => {
      if (active) setResources(data);
    }).catch(() => {});
    return () => { active = false; };
  }, [postId]);
  const legacy = splitMeetBody(body);
  const official = meetExternalURL(resources?.official_url || legacy.official_url);
  const timetable = meetExternalURL(resources?.timetable_url ?? "");
  const videoID = /^[a-zA-Z0-9_-]{11}$/.test(resources?.youtube_video_id ?? "") ? resources!.youtube_video_id : "";
  return <div className="space-y-4">
    {legacy.text ? <div className="whitespace-pre-wrap [overflow-wrap:anywhere] text-base leading-relaxed text-slate-200">{legacy.text}</div> : null}
    <div className="flex flex-col gap-3">
      {official ? <a href={official} target="_blank" rel="noopener noreferrer" className={linkClass}>大会の公式サイト</a> : null}
      {timetable ? <a href={timetable} target="_blank" rel="noopener noreferrer" className={linkClass}>タイムテーブル</a> : null}
    </div>
    {videoID ? <section aria-label="YouTube生配信" className="space-y-2">
      <h2 className="text-base font-semibold text-slate-100">YouTube生配信</h2>
      <YouTubePlayer videoId={videoID} title="大会のYouTube生配信" />
      <a href={`https://www.youtube.com/watch?v=${videoID}`} target="_blank" rel="noopener noreferrer" className="text-sm text-blue-300 underline underline-offset-4">YouTubeで開く</a>
    </section> : null}
  </div>;
}
