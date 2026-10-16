import type { Metadata } from "next";
import Link from "next/link";
import { coachApplyPath } from "@/lib/coach-recruit";

export const metadata: Metadata = {
  title: "オンラインコーチング",
  description: "TrackTalk のオンラインコーチングは準備中です。動画添削とパーソナル指導の説明と、コーチの募集です。",
};

const reviewSteps = ["動画を送る", "コーチが見る", "返事が届く"] as const;

// メニューは決まっているが、金額はまだ決めていないので載せない。
const prices = [
  { menu: "動画添削(コーチ)", price: "1本ごと" },
  { menu: "動画添削(先輩)", price: "コーチより安め" },
  { menu: "急ぎ", price: "追加料金" },
  { menu: "パーソナル指導", price: "コーチごと" },
] as const;

const tag = "text-xs text-slate-300";

// オンラインコーチングを始める前に、中身の説明とコーチの募集だけを出す。機能そのものはまだ置かない。
// 読む気が失せないよう、見出しと短い1行、要点だけにする。
export default function CoachingPage() {
  return (
    <div className="mx-auto max-w-md space-y-6">
      <div className="space-y-2">
        <p className="inline-flex rounded-full border border-blue-400/40 bg-blue-500/15 px-2.5 py-0.5 text-xs font-semibold text-blue-300">
          準備中
        </p>
        <h1 className="text-2xl font-bold">オンラインコーチング</h1>
        <p className="text-sm text-slate-200">コーチや先輩に、オンラインで走りを見てもらえます。</p>
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
          <li>集客・支払いは運営におまかせ</li>
          <li>大学生・社会人は「先輩」として登録可能</li>
        </ul>
        <Link
          href={coachApplyPath}
          className="flex min-h-11 w-full items-center justify-center rounded-md bg-blue-600 px-4 text-sm font-semibold text-white hover:bg-blue-500"
        >
          応募する
        </Link>
      </section>

      <section aria-labelledby="coaching-review" className="space-y-3">
        <h2 id="coaching-review" className="text-lg font-bold">
          動画添削
        </h2>
        <ol className="grid grid-cols-3 gap-2">
          {reviewSteps.map((step, index) => (
            <li key={step} className="flex flex-col items-center gap-1.5 rounded-xl border border-white/20 glass bg-white/10 p-3 text-center">
              <span
                className="flex size-7 items-center justify-center rounded-full bg-blue-600 text-sm font-bold text-white"
                aria-hidden
              >
                {index + 1}
              </span>
              <span className="text-xs font-semibold leading-snug text-white">{step}</span>
            </li>
          ))}
        </ol>
        <p className={tag}>夜に送れば、翌日の昼までに返ってきます。</p>
        <p className={tag}>最初は100mだけです。コーチが増えしだい、種目を増やします。</p>
        <p className={tag}>18歳未満は保護者の同意が必要です。</p>
        <p className={tag}>
          招待コードをお持ちの方は
          <Link href="/coaching/new" className="text-blue-300 underline">
            デモ版
          </Link>
          を使えます。
        </p>
      </section>

      <section aria-labelledby="coaching-personal" className="space-y-1">
        <h2 id="coaching-personal" className="text-lg font-bold">
          パーソナル指導
        </h2>
        <p className="text-sm text-slate-200">選んだコーチに、1対1で続けて見てもらえます。</p>
      </section>

      <section aria-labelledby="coaching-price" className="space-y-3">
        <h2 id="coaching-price" className="text-lg font-bold">
          料金
        </h2>
        <dl className="divide-y divide-white/15 overflow-hidden rounded-xl border border-white/20 glass bg-white/10">
          {prices.map((item) => (
            <div key={item.menu} className="flex items-baseline justify-between gap-3 px-3 py-2.5">
              <dt className="text-sm font-semibold text-white">{item.menu}</dt>
              <dd className="shrink-0 text-sm text-blue-300">{item.price}</dd>
            </div>
          ))}
        </dl>
        <p className={tag}>返事が目安の時刻より遅れたら自動で返金。</p>
      </section>
    </div>
  );
}
