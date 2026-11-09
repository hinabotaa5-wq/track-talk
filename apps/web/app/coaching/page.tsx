import type { Metadata } from "next";
import Link from "next/link";
import { ArrowRight, ClipboardCheck, Shuffle, UserRound, Users } from "lucide-react";
import { coachingCoachListEnabled, coachingEnabled } from "@/lib/features";
import { videoReviewStartNotice } from "@/lib/video-review-start";

// 開始予定日の案内は、ビルド時ではなくアクセス時の日時で出し分ける。
export const dynamic = "force-dynamic";

export const metadata: Metadata = {
  title: "コーチング",
  description: "コーチングは準備中です。現在は陸上競技のコーチを募集しています。",
};

const choices = [
  {
    title: "動画添削プラン",
    description: "コーチを選んで動画を送る",
    href: "/coaching/new",
    icon: Shuffle,
  },
  {
    title: "サポートプラン",
    description: "大会サポート・対面指導を選ぶ",
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
  const startNotice = videoReviewStartNotice();
  return (
    <div className="mx-auto flex min-h-[calc(100svh-10.5rem-env(safe-area-inset-top)-env(safe-area-inset-bottom))] max-w-md flex-col gap-4">
      {/* コーチ一覧はタブの一番上に置く。一覧を閉じている間は中身を見せず、押しても先へ進まない準備中の枠だけを出す。
          一覧フラグを開けたときだけ /coaches へのリンクにする(コーチング本体のフラグでは変えない)。 */}
      {startNotice ? (
        <p className="flex min-h-20 items-center justify-center rounded-2xl border border-blue-400/40 bg-blue-500/10 px-5 py-4 text-center text-lg font-bold text-slate-100">
          11/14(土)に開始します！
        </p>
      ) : coachingCoachListEnabled ? <Link href="/coaches" aria-labelledby="coach-list-heading" className="group flex min-w-0 items-center gap-4 rounded-2xl border border-white/20 glass bg-white/10 px-5 py-4 transition hover:border-blue-400/50 hover:bg-blue-500/15 active:scale-[0.98]">
        <span className="flex size-11 shrink-0 items-center justify-center rounded-xl bg-blue-500/20 text-blue-300">
          <Users className="size-6" aria-hidden />
        </span>
        <div className="min-w-0 flex-1">
          <h2 id="coach-list-heading" className="text-base font-bold leading-tight text-white">コーチ一覧</h2>
          <p className="mt-2 text-sm leading-snug text-slate-300">登録しているコーチを見る</p>
        </div>
        <ArrowRight className="size-4 shrink-0 text-blue-300" aria-hidden />
      </Link> : <section aria-labelledby="coach-list-heading" className="flex min-w-0 items-center gap-4 rounded-2xl border border-dashed border-white/25 bg-white/5 px-5 py-4">
        <span className="flex size-11 shrink-0 items-center justify-center rounded-xl bg-white/10 text-slate-300">
          <Users className="size-6" aria-hidden />
        </span>
        <div className="min-w-0 flex-1">
          <h2 id="coach-list-heading" className="text-base font-bold leading-tight text-slate-100">コーチ一覧</h2>
          <p className="mt-2 text-sm leading-snug text-slate-300">準備中です</p>
        </div>
      </section>}
      {!coachingEnabled ? <div className="space-y-1">
        <h1 className="text-xl font-bold">コーチングは準備中です</h1>
        {startNotice && <p className="text-sm text-slate-300">{startNotice}</p>}
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
              <p className="mt-2 text-sm leading-snug text-slate-300">{!coachingEnabled ? href === "/coach" ? "コーチ募集・応募状況の確認" : "準備中・公開までお待ちください" : href === "/coaches" && !coachingCoachListEnabled ? "準備中・公開までお待ちください" : description}</p>
            </div>
            <ArrowRight className="size-4 shrink-0 text-blue-300" aria-hidden />
          </Link>
        ))}
      </nav>

      {coachingEnabled ? <section className="space-y-3">
        <h2 className="text-sm font-semibold text-slate-100">利用中の方</h2>
        <nav aria-label="コーチングの利用状況" className="divide-y divide-white/15 overflow-hidden rounded-xl border border-white/20 bg-white/5">
          {[
            ["/me/coaching/tickets", "コーチングチケット"],
            ["/me/coaching/requests", "依頼した動画添削"],
            ["/coaching/support-consults", "大会までプランの無料相談"],
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
