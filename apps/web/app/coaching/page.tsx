import type { Metadata } from "next";
import Link from "next/link";
import { ArrowRight, GraduationCap, Shuffle, UserRound } from "lucide-react";

export const metadata: Metadata = {
  title: "コーチング",
  description: "ランダムで動画添削を依頼するか、コーチを指名して動画添削・大会サポート・対面指導を受けられます。",
};

const choices = [
  {
    title: "ランダムプラン",
    description: "おまかせで動画添削",
    href: "/coaching/new",
    icon: Shuffle,
  },
  {
    title: "指名プラン",
    description: "コーチとプランを選ぶ",
    href: "/coaches",
    icon: UserRound,
  },
  {
    title: "コーチ画面",
    description: "依頼・プランを管理",
    href: "/coach",
    icon: GraduationCap,
  },
] as const;

export default function CoachingPage() {
  return (
    <div className="mx-auto max-w-md space-y-6">

      <nav aria-label="コーチングのプランを選ぶ" className="space-y-2">
        {choices.map(({ title, description, href, icon: Icon }) => (
          <Link key={href} href={href} className="group flex min-w-0 items-center gap-3 rounded-xl border border-white/20 bg-white/10 px-4 py-3 transition hover:border-blue-400/50 hover:bg-blue-500/15 active:scale-[0.98]">
            <span className="flex size-8 shrink-0 items-center justify-center rounded-lg bg-blue-500/20 text-blue-300">
              <Icon className="size-4" aria-hidden />
            </span>
            <div className="min-w-0 flex-1">
              <h2 className="text-sm font-bold leading-tight text-white">{title}</h2>
              <p className="mt-1 text-xs leading-snug text-slate-300">{description}</p>
            </div>
            <ArrowRight className="size-4 shrink-0 text-blue-300" aria-hidden />
          </Link>
        ))}
      </nav>

      <section className="space-y-3">
        <h2 className="text-sm font-semibold text-slate-100">利用中の方</h2>
        <nav aria-label="コーチングの利用状況" className="divide-y divide-white/15 overflow-hidden rounded-xl border border-white/20 bg-white/5">
          {[
            ["/me/coaching/tickets", "チケットの残高・購入"],
            ["/me/coaching/requests", "依頼した動画添削"],
            ["/me/coaching/personal", "申し込んだ指名プラン"],
          ].map(([href, label]) => (
            <Link key={href} href={href} className="flex min-h-12 items-center justify-between gap-3 px-4 text-sm text-slate-200 transition hover:bg-white/10">
              {label}<ArrowRight className="size-4 text-slate-400" aria-hidden />
            </Link>
          ))}
        </nav>
      </section>
    </div>
  );
}
