import type { Metadata } from "next";
import Link from "next/link";
import { ArrowRight, Shuffle, UserRound, Video, Trophy, MapPin } from "lucide-react";

export const metadata: Metadata = {
  title: "コーチ機能について",
  description: "陸上の経験を、次の一歩を支えるアドバイスに。TrackTalkのコーチの役割と、動画添削・サポートプランの仕組みを紹介します。",
};

export default function CoachAboutPage() {
  return (
    <div className="mx-auto max-w-md space-y-7">
      <section className="space-y-3">
        <p className="text-sm font-semibold text-blue-300">TrackTalkのコーチ機能</p>
        <p className="rounded-xl border border-blue-300/30 bg-blue-500/15 p-3 text-sm font-semibold text-blue-200">現在はコーチ募集のみです。以下は公開後に提供する機能の紹介です。指導の申込み・チケットの購入はまだできません。</p>
        <h1 className="text-2xl font-bold leading-relaxed">あなたの陸上経験を、<br />誰かの次の一歩に。</h1>
        <p className="text-sm leading-relaxed text-slate-200">「フォームを見てほしい」「大会に向けて何をすればいい？」。そんな選手の悩みに、競技や指導で培った経験を生かして応える機能です。動画でのアドバイスから継続的な指導まで、自分が対応できる形で活動できます。</p>
      </section>

      <section className="space-y-4" aria-labelledby="coach-ways">
        <h2 id="coach-ways" className="text-lg font-bold">関わり方は、大きく2つ</h2>
        <article className="space-y-3 rounded-2xl border border-blue-400/30 bg-blue-500/10 p-4">
          <h3 className="flex items-center gap-2 font-bold"><Shuffle className="size-5 text-blue-300" aria-hidden />動画添削プラン：動画1本から支える</h3>
          <p className="text-sm leading-relaxed text-slate-200">利用者が種目と相談内容、動画を送ると、担当種目に合う受付中のコーチへ順番に依頼が届きます。利用者がコーチを選ぶ必要がないので、気軽に相談を始められます。</p>
          <p className="text-sm leading-relaxed text-slate-200">コーチは届いた依頼を確認し、対応できるものを引き受けます。動画を見て、良い点・直す点・おすすめのドリルを返し、選手が次の練習で試せる具体的な一歩を伝えます。</p>
        </article>
        <article className="space-y-3 rounded-2xl border border-white/20 bg-white/10 p-4">
          <h3 className="flex items-center gap-2 font-bold"><UserRound className="size-5 text-blue-300" aria-hidden />サポートプラン：あなたに相談したい人を支える</h3>
          <p className="text-sm leading-relaxed text-slate-200">コーチが指導内容・料金・期間・定員を決めてプランを公開し、利用者はプロフィールや内容を見て申し込みます。申し込みをコーチが承諾した後、必要な支払い・同意を経て指導が始まります。</p>
          <ul className="space-y-3 text-sm text-slate-200">
            <li className="flex gap-2"><Video className="mt-0.5 size-4 shrink-0 text-blue-300" aria-hidden /><span><strong className="text-white">動画添削</strong>：動画やメッセージを通して、フォームや練習をサポート。</span></li>
            <li className="flex gap-2"><Trophy className="mt-0.5 size-4 shrink-0 text-blue-300" aria-hidden /><span><strong className="text-white">大会までのサポート</strong>：目標の大会に向けて、準備や調整をサポート。</span></li>
            <li className="flex gap-2"><MapPin className="mt-0.5 size-4 shrink-0 text-blue-300" aria-hidden /><span><strong className="text-white">対面指導</strong>：場所や回数などをプランに明記し、現地で指導。</span></li>
          </ul>
        </article>
      </section>

      <section className="space-y-3">
        <h2 className="text-lg font-bold">自分のペースで、責任を持って</h2>
        <p className="text-sm leading-relaxed text-slate-200">ランダム添削は受付のオン・オフや1日の上限を設定できます。サポートプランは対応できる内容と定員を自分で決めます。引き受けた依頼では期限を守り、選手の経験や目標に合わせて、分かりやすい言葉で伝えてください。</p>
        <p className="text-sm leading-relaxed text-slate-200">やり取りや支払いはTrackTalk上で行います。未成年の利用には保護者の同意など、所定の手続きが必要です。</p>
      </section>

      <section className="space-y-3">
        <h2 className="text-lg font-bold">報酬について</h2>
        <p className="text-sm leading-relaxed text-slate-200">有料の依頼やプランで指導を提供すると、サービスの手数料を差し引いた報酬が記録されます。受け取り口座の登録、適用される手数料、報酬と送金の状況は、コーチ専用画面で確認できます。</p>
      </section>

      <section className="space-y-3">
        <h2 className="text-lg font-bold">応募から活動まで</h2>
        <ol className="list-decimal space-y-2 pl-5 text-sm leading-relaxed text-slate-200">
          <li>名前、担当種目、記録、成績や指導経験、顔写真と、SNSまたは所属チームのホームページのURLを入力します。URLは1〜3件必須です。</li>
          <li>運営が内容を確認します。審査状況は応募画面で確認できます。</li>
          <li>承認後も公開までは準備中です。「コーチ」タブの「コーチ専用画面」から、承認状況や顔写真を確認できます。活動開始の方法は公開時に案内します。</li>
        </ol>
      </section>
      <Link href="/coach/apply" replace className="flex min-h-12 items-center justify-between gap-3 rounded-xl bg-blue-600 px-4 py-3 text-sm font-semibold text-white transition hover:bg-blue-500">
        応募画面へ進む<ArrowRight className="size-4" aria-hidden />
      </Link>
    </div>
  );
}
