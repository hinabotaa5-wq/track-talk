"use client";

import Link from "next/link";
import { ChevronRight, Flame } from "lucide-react";
import { useEffect, useState } from "react";
import { apiFetch } from "@/lib/api";
import {
  formatRemaining,
  menuContestDeadline,
  menuContestName,
  remainingParts,
  type MenuContestState,
} from "@/lib/menu-contest";

// 残り時間は分まで出すので、30秒ごとに描き直せば表示が遅れても30秒以内に収まる。
export function useNow(intervalMs = 30_000) {
  const [now, setNow] = useState(() => Date.now());
  useEffect(() => {
    const id = window.setInterval(() => setNow(Date.now()), intervalMs);
    return () => window.clearInterval(id);
  }, [intervalMs]);
  return now;
}

// ホームのヘッダーの下段。エグ練大会の開催中だけ、いまの段階と締め切りまでの残り時間を出す。
export function MenuContestBar() {
  const [state, setState] = useState<MenuContestState | null>(null);
  const [reload, setReload] = useState(0);
  const now = useNow();

  useEffect(() => {
    const controller = new AbortController();
    apiFetch<MenuContestState>("/v1/menu-contest", { signal: controller.signal })
      .then(setState)
      .catch(() => {
        // 取れなければ出さないだけ。
      });
    return () => controller.abort();
  }, [reload]);

  const round = state?.round;
  const deadline = round ? menuContestDeadline(round) : null;
  const parts = deadline ? remainingParts(Date.parse(deadline.until), now) : null;

  // 締め切りを過ぎたら、次の段階を取り直す。
  useEffect(() => {
    if (deadline && !parts) {
      const id = window.setTimeout(() => setReload((n) => n + 1), 1000);
      return () => window.clearTimeout(id);
    }
  }, [deadline, parts]);

  if (!round || !deadline || !parts) return null;

  return (
    <Link
      href="/menu-contest"
      className="mx-auto flex max-w-5xl items-center gap-2 px-4 pb-2 text-sm transition active:scale-[0.98]"
    >
      <span
        className={`flex min-w-0 flex-1 items-center gap-2 rounded-full border px-3 py-1.5 ${
          parts.urgent ? "border-red-300/60 bg-red-500/20" : "border-orange-300/50 bg-orange-500/15"
        }`}
      >
        <Flame className={`size-4 shrink-0 ${parts.urgent ? "text-red-300" : "text-orange-300"}`} aria-hidden />
        <span className="shrink-0 font-bold text-white">{menuContestName}</span>
        <span className="min-w-0 flex-1 truncate text-right text-slate-200">
          <span className="text-xs">{deadline.prefix} </span>
          <span className={`font-bold tabular-nums ${parts.urgent ? "text-red-200" : "text-orange-200"}`}>
            {formatRemaining(parts)}
          </span>
        </span>
        <ChevronRight className="size-4 shrink-0 text-slate-300" aria-hidden />
      </span>
    </Link>
  );
}
