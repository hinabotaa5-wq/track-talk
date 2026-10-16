import type { Metadata } from "next";
import Link from "next/link";
import { coachApplyPath } from "@/lib/coach-recruit";

export const metadata: Metadata = {
  title: "オンラインコーチング",
  description: "TrackTalk のオンラインコーチングは準備中です。動画添削とパーソナル指導の説明と、コーチの募集です。",
};

const reviewSteps = [
  { title: "走りの動画を送る", body: "スマホで撮った練習やレースの短い動画を送ります。" },
  { title: "コーチが見る", body: "そのとき受け付けているコーチに、自動で回ります。コーチを選ぶ手間はありません。" },
  { title: "アドバイスが届く", body: "良いところ、直すところ、おすすめの練習を、決まった形で返します。" },
] as const;

// オンラインコーチングを始める前に、中身の説明とコーチの募集だけを出す。機能そのものはまだ置かない。
export default function CoachingPage() {
  return (
    <div className="mx-auto max-w-md space-y-8">
      <div className="space-y-2">
        <p className="inline-flex rounded-full border border-blue-400/40 bg-blue-500/15 px-2.5 py-0.5 text-xs font-semibold text-blue-300">
          準備中
        </p>
        <h1 className="text-2xl font-bold">オンラインコーチング</h1>
        <p className="text-sm leading-7 text-slate-200">
          近くに教えてくれる人がいなくても、陸上のコーチや先輩に、オンラインで走りを見てもらえる場所を準備しています。始まったら、このタブからお知らせします。
        </p>
      </div>

      <section
        aria-labelledby="coach-recruit-title"
        className="space-y-3 rounded-2xl border border-blue-400/40 glass bg-blue-500/15 p-4"
      >
        <h2 id="coach-recruit-title" className="text-lg font-bold text-white">
          コーチを募集しています
        </h2>
        <p className="text-sm leading-7 text-slate-200">
          動画添削やオンライン指導をしてくれる方を探しています。YouTube や SNS で教えている方も歓迎です。競技経験のある大学生・社会人の方は「先輩」として応募できます。
        </p>
        <ul className="list-disc space-y-1 pl-5 text-xs leading-6 text-slate-200">
          <li>受け付けたい時間だけ「受付中」にして、届いた動画にアドバイスを返します。1本ごとに報酬があります。</li>
          <li>集客、支払い、保護者とのやりとりは TrackTalk が引き受けます。</li>
          <li>運営が一人ずつ確認してから参加していただきます。</li>
        </ul>
        <Link
          href={coachApplyPath}
          className="flex min-h-11 w-full items-center justify-center rounded-md bg-blue-600 px-4 text-sm font-semibold text-white hover:bg-blue-500"
        >
          応募する
        </Link>
        <p className="text-xs text-slate-300">お問い合わせフォームから応募できます(ログインが必要です)。</p>
      </section>

      <section aria-labelledby="coaching-review" className="space-y-3">
        <div className="flex items-baseline justify-between gap-2">
          <h2 id="coaching-review" className="text-lg font-bold">
            動画添削
          </h2>
          <span className="text-xs text-blue-300">最初に始めます</span>
        </div>
        <p className="text-sm leading-7 text-slate-200">
          自分の走りの動画を送ると、コーチがフォームを見てアドバイスを返す、1本ごとの有料サービスです。
        </p>
        <ol className="space-y-2">
          {reviewSteps.map((step, index) => (
            <li key={step.title} className="flex gap-3 rounded-xl border border-white/20 glass bg-white/10 p-3">
              <span
                className="flex size-7 shrink-0 items-center justify-center rounded-full bg-blue-600 text-sm font-bold text-white"
                aria-hidden
              >
                {index + 1}
              </span>
              <div className="min-w-0 space-y-0.5">
                <p className="text-sm font-semibold text-white">{step.title}</p>
                <p className="text-xs leading-snug text-slate-200">{step.body}</p>
              </div>
            </li>
          ))}
        </ol>
        <ul className="list-disc space-y-1 pl-5 text-xs leading-6 text-slate-300">
          <li>
            見てもらう相手は、指導者の「コーチ」と、競技経験のある大学生・社会人の「先輩」から選べます。先輩は年が近く、同じ悩みを経験した人です。
          </li>
          <li>やりとりは動画とアドバイスの一往復だけです。コーチと個別にメッセージを送りあうことはありません。</li>
          <li>18歳未満の人は、保護者の同意を得てから使います。</li>
        </ul>
      </section>

      <section aria-labelledby="coaching-personal" className="space-y-3">
        <div className="flex items-baseline justify-between gap-2">
          <h2 id="coaching-personal" className="text-lg font-bold">
            パーソナル指導
          </h2>
          <span className="text-xs text-slate-300">あとで始めます</span>
        </div>
        <p className="text-sm leading-7 text-slate-200">
          コーチのプロフィールを見比べて自分で選び、一定の期間、1対1で継続して見てもらうサービスです。料金はコーチごとに決まります。18歳未満の人のやりとりは、保護者も見られます。
        </p>
      </section>
    </div>
  );
}
