"use client";
import { L, splitAround } from "@/lib/i18n";

import Link from "next/link";
import { Plus } from "lucide-react";
import { todayInTokyo } from "@/lib/format";
import { targetMeetCountdown, useTargetMeet } from "@/lib/target-meet";
import { useNow } from "@/lib/use-now";

// メモタブの瓶の横。セットした大会まであと何日かを出し、タップでその大会の一覧へ(別の大会への変更・解除)。
// セットしていなければ「+」で大会タブへ。県別の大会一覧は #549 で廃止したため、
// competition の目標は国際大会のタブで変更・解除する。
export function TargetMeetCountdown({ token }: { token: string | null }) {
  const { target, loaded } = useTargetMeet(token);
  const now = useNow();
  if (!loaded || now === null) return null;

  if (!target) {
    return (
      <Link
        href="/competitions"
        className="flex items-center gap-2 rounded-xl border border-dashed border-white/30 bg-white/5 px-3 py-2 text-xs font-semibold text-slate-200 transition hover:bg-white/10 active:scale-[0.98]"
      >
        <span className="flex size-7 shrink-0 items-center justify-center rounded-full bg-sky-400/25 text-sky-100">
          <Plus className="size-4" aria-hidden />
        </span>{L("目標の大会をセット")}</Link>
    );
  }

  const countdown = targetMeetCountdown(target, todayInTokyo(new Date(now)));
  const daysLeft = splitAround("まであと {days} 日", "days");
  const href = target.source === "major" ? "/competitions" : "/competitions?view=international";
  return (
    <Link
      href={href}
      aria-label={L("{name}。大会一覧で変更・解除する", { name: target.name })}
      className="block rounded-xl border border-sky-300/30 bg-sky-950/40 px-3 py-2 transition hover:bg-sky-900/40 active:scale-[0.98]"
    >
      <span className="line-clamp-2 text-xs font-semibold text-sky-100">{target.name}</span>
      <span className="mt-0.5 block text-sm text-slate-100">
        {countdown.kind === "before" ? (
          <>{daysLeft[0]}<span className="text-2xl font-bold tabular-nums text-white">{countdown.days}</span>{daysLeft[1]}</>
        ) : countdown.kind === "today" ? (
          <span className="font-bold text-amber-200">{L("今日が本番！")}</span>
        ) : countdown.kind === "ongoing" ? (
          <span className="font-bold text-amber-200">{L("開催中")}</span>
        ) : (
          <span className="text-slate-300">{L("終了しました。次の大会をセット")}</span>
        )}
      </span>
    </Link>
  );
}
