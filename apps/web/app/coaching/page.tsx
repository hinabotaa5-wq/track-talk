import type { Metadata } from "next";
import Link from "next/link";
import { coachApplyPath } from "@/lib/coach-recruit";
import { coachingEnabled } from "@/lib/features";

export const metadata: Metadata = {
  title: "オンラインコーチング",
  description: "TrackTalk のオンラインコーチングは準備中です。自分の走りの動画をコーチに見てもらえます。料金と、コーチの募集です。",
};

// 1本の値段は API の reviewPrices(review_payment.go)と同じにする。
const prices = [
  { menu: "動画1本(コーチ)", price: "1,000円" },
  { menu: "動画1本(先輩)", price: "500円" },
] as const;

const tag = "text-xs text-slate-300";

// オンラインコーチングを始める前に、中身の説明とコーチの募集だけを出す。機能そのものはまだ置かない。
// 読む気が失せないよう、見出しと短い1行、要点だけにする。
export default function CoachingPage() {
  return (
    <div className="mx-auto max-w-md space-y-6">
      <div className="space-y-3">
        <p className="flex min-h-14 items-center justify-center rounded-2xl border-2 border-amber-300/70 bg-amber-400/20 px-4 text-2xl font-extrabold tracking-widest text-amber-200">
          準備中
        </p>
        <h1 className="text-2xl font-bold">オンラインコーチング</h1>
        <p className="text-sm text-slate-200">自分の走りの動画をコーチに見てもらえます</p>
      </div>

      <section
        aria-labelledby="coach-recruit-title"
        className="space-y-3 rounded-2xl border border-blue-400/40 glass bg-blue-500/15 p-4"
      >
        <h2 id="coach-recruit-title" className="text-lg font-bold text-white">
          コーチ募集中
        </h2>
        <ul className="list-disc space-y-0.5 pl-5 text-sm text-slate-200">
          <li>動画1本ごとに報酬</li>
          <li>コーチング経験のある方</li>
          <li>社会人で競技経験が豊富な方</li>
          <li>大学生で成績が優秀な方</li>
          <li>応募完了後は運営が審査します</li>
        </ul>
        <Link
          href={coachApplyPath}
          className="flex min-h-11 w-full items-center justify-center rounded-md bg-blue-600 px-4 text-sm font-semibold text-white hover:bg-blue-500"
        >
          応募する
        </Link>
      </section>

      <section aria-labelledby="coaching-price" className="space-y-3">
        <h2 id="coaching-price" className="text-lg font-bold">
          料金
        </h2>
        <dl className="divide-y divide-white/15 overflow-hidden rounded-xl border border-white/20 glass bg-white/10">
          {prices.map((item) => (
            <div key={item.menu} className="flex items-baseline justify-between gap-3 px-3 py-2.5">
              <dt className="text-sm font-semibold text-white">{item.menu}</dt>
              <dd className="shrink-0 text-sm font-semibold text-blue-300">{item.price}</dd>
            </div>
          ))}
        </dl>
        <p className={tag}>返事が目安の時刻より遅れたら自動で返金されます</p>
        {coachingEnabled ? (
          <p className={tag}>
            <Link href="/coaches" className="text-blue-300 underline">
              コーチ一覧
            </Link>
            ・招待コードをお持ちの方は
            <Link href="/coaching/new" className="text-blue-300 underline">
              デモ版
            </Link>
          </p>
        ) : null}
      </section>
    </div>
  );
}
