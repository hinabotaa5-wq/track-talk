"use client";

import { MessageCircle, Trophy, X } from "lucide-react";
import Link from "next/link";
import { useCallback, useEffect, useId, useRef, useState } from "react";
import { useAuth } from "./auth-provider";
import { AnchoredPanel } from "./anchored-panel";
import { fetchTodayMeetThreads } from "@/lib/meet-threads";
import { todayInTokyo } from "@/lib/format";
import type { MeetThread } from "@/lib/types";

// 閉じた日を覚えておく。アプリを開き直す(タブやホーム画面アプリを起動し直す)とまた出る。
const closedKey = "today-meet-balloon:closed";

function closedToday(today: string) {
  try {
    return window.sessionStorage.getItem(closedKey) === today;
  } catch {
    return false;
  }
}

function rememberClosed(today: string) {
  try {
    window.sessionStorage.setItem(closedKey, today);
  } catch {
    // 保存できなくても、閉じること自体はできる。
  }
}

export function TodayMeetBalloon() {
  const { token } = useAuth();
  const panelID = useId();
  // 開いた状態で初めて描くとき、外側の div より先にこのボタンの ref が付くので、位置合わせはボタンに合わせる。
  const buttonRef = useRef<HTMLButtonElement>(null);
  const [meets, setMeets] = useState<MeetThread[]>([]);
  const [open, setOpen] = useState(false);

  useEffect(() => {
    let active = true;
    fetchTodayMeetThreads(token).then((next) => {
      if (!active) return;
      setMeets(next);
      if (next.length > 0 && !closedToday(todayInTokyo())) setOpen(true);
    });
    return () => {
      active = false;
    };
  }, [token]);

  const close = useCallback(() => {
    rememberClosed(todayInTokyo());
    setOpen(false);
  }, []);

  if (meets.length === 0) return null;

  return (
    <div className="relative">
      <button
        ref={buttonRef}
        type="button"
        aria-label="今日開催中の大会"
        aria-expanded={open}
        aria-controls={panelID}
        onClick={() => (open ? close() : setOpen(true))}
        className="relative inline-flex size-11 items-center justify-center rounded-full text-amber-300 transition hover:bg-white/16 active:scale-[0.98]"
      >
        <Trophy className="size-5" strokeWidth={2} aria-hidden />
        <span className="absolute top-2 right-2 size-2 rounded-full bg-amber-400 ring-2 ring-[#16243a]" aria-hidden />
      </button>
      {/* 吹き出しのしっぽ。トロフィーの真下に置き、下半分は吹き出しの本体が覆う。 */}
      {open ? (
        <span
          className="pointer-events-none absolute top-[calc(100%+5px)] left-1/2 size-3 -translate-x-1/2 rotate-45 border-t border-l border-amber-200/60 bg-amber-50"
          aria-hidden
        />
      ) : null}
      <AnchoredPanel
        open={open}
        anchorRef={buttonRef}
        panelID={panelID}
        onClose={close}
        // 右の2つのアイコンの分だけ右へ寄せ、ヘッダーの右端にそろえる。スマホでも左にはみ出さない。
        rightShiftRem={6}
        className="w-80 max-w-[calc(100vw-2rem)] mt-2 rounded-2xl border border-amber-200/60 bg-amber-50 text-slate-900 shadow-xl"
      >
        <div role="dialog" aria-label="今日開催中の大会" className="relative p-4">
          <button
            type="button"
            onClick={close}
            aria-label="閉じる"
            className="absolute top-2 right-2 inline-flex size-8 items-center justify-center rounded-full text-slate-500 transition hover:bg-amber-100 active:scale-[0.98]"
          >
            <X className="size-4" aria-hidden />
          </button>
          <p className="pr-6 text-sm font-bold leading-relaxed">今日は以下の大会が開催されてるよ！</p>
          <ul className="mt-3 space-y-2">
            {meets.map((meet) => (
              <li key={meet.meet_key}>
                <Link
                  href={meet.href}
                  onClick={close}
                  className="block rounded-xl bg-white px-3 py-2 shadow-xs transition hover:bg-amber-100 active:scale-[0.98]"
                >
                  <span className="flex items-start justify-between gap-2">
                    <span className="min-w-0">
                      <span className="block truncate text-sm font-semibold text-blue-700">{meet.name}</span>
                      <span className="block text-xs text-slate-500">{meet.date_label}</span>
                    </span>
                    <span className="flex shrink-0 items-center gap-1 text-xs text-slate-500">
                      <MessageCircle className="size-3.5" aria-hidden />
                      {meet.comment_count}
                    </span>
                  </span>
                </Link>
              </li>
            ))}
          </ul>
        </div>
      </AnchoredPanel>
    </div>
  );
}
