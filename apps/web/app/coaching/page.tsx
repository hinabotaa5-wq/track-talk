import type { Metadata } from "next";
import Link from "next/link";
import { ArrowRight, Shuffle, UserRound } from "lucide-react";
import { coachApplyPath } from "@/lib/coach-recruit";

export const metadata: Metadata = {
  title: "コーチング",
  description: "ランダムで動画添削を依頼するか、コーチを指名して動画添削・大会サポート・対面指導を受けられます。",
};

const choices = [
  {
    title: "ランダムプラン",
    description: "コーチ選びはおまかせ。種目に合う受付中のコーチに、動画添削やアドバイスを依頼できます。",
    href: "/coaching/new",
    action: "動画添削を依頼する",
    icon: Shuffle,
  },
  {
    title: "指名プラン",
    description: "コーチと料金を見て自分で選べます。動画添削・大会までのサポート・対面指導から探せます。",
    href: "/coaches",
    action: "コーチとプランを選ぶ",
    icon: UserRound,
  },
] as const;

export default function CoachingPage() {
  return (
    <div className="mx-auto max-w-md space-y-6">

      <nav aria-label="コーチングのプランを選ぶ" className="space-y-4">
        {choices.map(({ title, description, href, action, icon: Icon }) => (
          <Link key={href} href={href} className="group block space-y-3 rounded-2xl border border-white/20 bg-white/10 p-5 transition hover:border-blue-400/50 hover:bg-blue-500/15 active:scale-[0.98]">
            <div className="flex items-center gap-3">
              <span className="flex size-10 items-center justify-center rounded-xl bg-blue-500/20 text-blue-300">
                <Icon className="size-5" aria-hidden />
              </span>
              <h2 className="text-xl font-bold text-white">{title}</h2>
            </div>
            <p className="text-sm leading-relaxed text-slate-200">{description}</p>
            <p className="flex items-center justify-between gap-2 text-sm font-semibold text-blue-300">
              {action}<ArrowRight className="size-4 transition group-hover:translate-x-1" aria-hidden />
            </p>
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

      <Link href={coachApplyPath} className="flex min-h-12 items-center justify-between gap-3 rounded-xl border border-white/20 bg-white/10 px-4 py-3 text-sm font-semibold text-slate-100 transition hover:border-blue-400/50 hover:bg-white/15 active:scale-[0.98]">
        コーチに応募する<ArrowRight className="size-4 text-blue-300" aria-hidden />
      </Link>
    </div>
  );
}
