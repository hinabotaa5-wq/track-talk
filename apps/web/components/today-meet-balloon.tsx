"use client";

import { ExternalLink, Trophy, X } from "lucide-react";
import { useCallback, useEffect, useId, useRef, useState } from "react";
import { loadCompetitions, readCompetitions } from "@/lib/competition-list";
import { loadDomesticMajors, peekDomesticMajors } from "@/lib/domestic-majors";
import { INTERNATIONAL } from "@/lib/prefectures";
import { todayInTokyo } from "@/lib/season-schedule";
import { todayMeets, type TodayMeet } from "@/lib/today-meets";
import { AnchoredPanel } from "./anchored-panel";
import { useAuth } from "./auth-provider";

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

function shownURL(href: string) {
  if (href.startsWith("/")) return "TrackTalk の大会タブ";
  return href.replace(/^https?:\/\//, "").replace(/\/$/, "");
}

export function TodayMeetBalloon() {
  const { token } = useAuth();
  const panelID = useId();
  // 開いた状態で初めて描くとき、外側の div より先にこのボタンの ref が付くので、位置合わせはボタンに合わせる。
  const buttonRef = useRef<HTMLButtonElement>(null);
  const [meets, setMeets] = useState<TodayMeet[]>([]);
  const [open, setOpen] = useState(false);

  useEffect(() => {
    if (!token) return;
    let active = true;
    const today = todayInTokyo();
    // 大会タブと同じ取得とキャッシュを使う。どちらかが取れなくても、取れた方だけで出す。
    Promise.all([
      loadDomesticMajors(token).catch(() => peekDomesticMajors() ?? []),
      loadCompetitions(INTERNATIONAL, 0, token).then(
        (entry) => entry.items,
        () => readCompetitions(INTERNATIONAL, 0)?.items ?? [],
      ),
    ]).then(([domestic, international]) => {
      if (!active) return;
      const next = todayMeets(domestic, international, today);
      setMeets(next);
      if (next.length > 0 && !closedToday(today)) setOpen(true);
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
        className="relative inline-flex size-11 items-center justify-center rounded-full text-amber-300 transition hover:bg-white/16 active:scale-95"
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
            className="absolute top-2 right-2 inline-flex size-8 items-center justify-center rounded-full text-slate-500 transition hover:bg-amber-100 active:scale-95"
          >
            <X className="size-4" aria-hidden />
          </button>
          <p className="pr-6 text-sm font-bold leading-relaxed">今日は以下の大会が開催されてるよ！</p>
          <ul className="mt-3 space-y-2">
            {meets.map((meet) => {
              const external = !meet.href.startsWith("/");
              return (
                <li key={meet.key}>
                  <a
                    href={meet.href}
                    target={external ? "_blank" : undefined}
                    rel={external ? "noopener noreferrer" : undefined}
                    onClick={close}
                    className="block rounded-xl bg-white px-3 py-2 shadow-xs transition hover:bg-amber-100 active:scale-95"
                  >
                    <span className="flex items-start gap-1 text-sm font-semibold text-blue-700">
                      <span className="line-clamp-2">{meet.name}</span>
                      <ExternalLink className="mt-0.5 size-3.5 shrink-0" aria-hidden />
                    </span>
                    <span className="block truncate text-xs text-slate-500">{shownURL(meet.href)}</span>
                  </a>
                </li>
              );
            })}
          </ul>
        </div>
      </AnchoredPanel>
    </div>
  );
}
