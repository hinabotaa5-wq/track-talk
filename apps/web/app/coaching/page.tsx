import type { Metadata } from "next";
import Link from "next/link";
import { ArrowRight, ClipboardCheck, Shuffle, UserRound } from "lucide-react";
import { coachingEnabled } from "@/lib/features";

export const metadata: Metadata = {
  title: "コーチング",
  description: "コーチングは準備中です。現在は陸上競技のコーチを募集しています。",
};

const choices = [
  {
    title: "動画添削プラン",
    description: "おまかせで動画添削",
    href: "/coaching/new",
    icon: Shuffle,
  },
  {
    title: "サポートプラン",
    description: "コーチとプランを選ぶ",
    href: "/coaches",
    icon: UserRound,
  },
  {
    title: "コーチ専用画面",
    description: "依頼・プランを管理",
    href: "/coach",
    icon: ClipboardCheck,
  },
] as const;

export default function CoachingPage() {
  return (
    <div className="mx-auto flex min-h-[calc(100svh-10.5rem-env(safe-area-inset-top)-env(safe-area-inset-bottom))] max-w-md flex-col gap-4">
      {!coachingEnabled ? <div className="space-y-1">
        <h1 className="text-xl font-bold">コーチングは準備中です</h1>
        <p className="text-sm text-slate-300">現在はコーチを募集しています。応募は「コーチ専用画面」から。</p>
      </div> : null}
      <nav aria-label="コーチングのプランを選ぶ" className="grid flex-1 auto-rows-fr gap-3">
        {choices.map(({ title, description, href, icon: Icon }) => (
          <Link key={href} href={href} className="group flex min-h-24 min-w-0 items-center gap-4 rounded-2xl border border-white/20 glass bg-white/10 px-5 py-5 transition hover:border-blue-400/50 hover:bg-blue-500/15 active:scale-[0.98]">
            <span className="flex size-11 shrink-0 items-center justify-center rounded-xl bg-blue-500/20 text-blue-300">
              <Icon className="size-6" aria-hidden />
            </span>
            <div className="min-w-0 flex-1">
              <h2 className="text-base font-bold leading-tight text-white">{title}</h2>
              <p className="mt-2 text-sm leading-snug text-slate-300">{!coachingEnabled ? href === "/coach" ? "コーチ募集・応募状況の確認" : "準備中・公開までお待ちください" : description}</p>
            </div>
            <ArrowRight className="size-4 shrink-0 text-blue-300" aria-hidden />
          </Link>
        ))}
      </nav>

      {coachingEnabled ? <section className="space-y-3">
        <h2 className="text-sm font-semibold text-slate-100">利用中の方</h2>
        <nav aria-label="コーチングの利用状況" className="divide-y divide-white/15 overflow-hidden rounded-xl border border-white/20 bg-white/5">
          {[
            ["/me/coaching/tickets", "チケットの残高・購入"],
            ["/me/coaching/requests", "依頼した動画添削"],
            ["/me/coaching/personal", "申し込んだサポートプラン"],
          ].map(([href, label]) => (
            <Link key={href} href={href} className="flex min-h-12 items-center justify-between gap-3 px-4 text-sm text-slate-200 transition hover:bg-white/10">
              {label}<ArrowRight className="size-4 text-slate-400" aria-hidden />
            </Link>
          ))}
        </nav>
      </section> : null}
    </div>
  );
}
