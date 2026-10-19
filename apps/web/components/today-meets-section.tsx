import Link from "next/link";
import { MessageCircle, Trophy } from "lucide-react";
import { NowRunning } from "@/components/meet-now-running";
import type { MeetThread } from "@/lib/types";

// ホームの「今日開催中」。実況スレッドへ誘導する。対象大会が無い日は親が描画しない。
export function TodayMeetsSection({ meets }: { meets: MeetThread[] }) {
  if (meets.length === 0) return null;

  return (
    <section
      aria-label="今日開催中の大会"
      className="mx-auto w-full max-w-5xl px-4 pt-4"
    >
      <div className="rounded-2xl border border-amber-200/60 bg-amber-50 p-4 text-slate-900 shadow-sm">
        <h2 className="flex items-center gap-2 text-sm font-bold">
          <Trophy className="size-4 text-amber-500" aria-hidden />
          今日開催中
        </h2>
        <ul className="mt-3 space-y-2">
          {meets.map((meet) => (
            <li key={meet.meet_key}>
              <Link
                href={meet.href}
                className="block rounded-xl bg-white px-3 py-2 shadow-xs transition hover:bg-amber-100 active:scale-[0.99]"
              >
                <span className="flex items-start justify-between gap-2">
                  <span className="min-w-0">
                    <span className="block truncate text-sm font-semibold text-blue-700">{meet.name}</span>
                    <span className="block text-xs text-slate-500">{meet.date_label}</span>
                    <NowRunning events={meet.now_running} className="mt-0.5 block text-xs font-semibold text-emerald-700" />
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
    </section>
  );
}
