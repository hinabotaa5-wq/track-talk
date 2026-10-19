"use client";

import { useEffect, useState } from "react";
import { fetchTodayMeetThreads } from "@/lib/meet-threads";

// 「いま実施中: 女子100m予選」の1行。実施中の種目が無ければ何も出さない。
export function NowRunning({ events, className }: { events?: string[]; className?: string }) {
  if (!events || events.length === 0) return null;
  return <span className={className}>いま実施中: {events.join("、")}</span>;
}

// 実況スレッドの上部に出す。スレッドの投稿は誰でも読めるので、今日の一覧から自分の投稿を探して表示する。
export function MeetThreadNowRunning({ postId }: { postId: string }) {
  const [events, setEvents] = useState<string[]>([]);

  useEffect(() => {
    let cancelled = false;
    fetchTodayMeetThreads().then((meets) => {
      if (cancelled) return;
      setEvents(meets.find((m) => m.post_id === postId)?.now_running ?? []);
    });
    return () => {
      cancelled = true;
    };
  }, [postId]);

  return (
    <NowRunning
      events={events}
      className="block rounded-xl border border-emerald-300/40 bg-emerald-500/15 px-3 py-2 text-sm font-semibold text-emerald-100"
    />
  );
}
