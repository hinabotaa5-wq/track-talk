"use client";

import { L } from "@/lib/i18n";
import { SupportPlanCoachTerms } from "@/components/support-plan-coach-terms";
import Link from "next/link";
import { ArrowRight, Shuffle, UserRound, Trophy, MapPin } from "lucide-react";


export function CoachAboutView() {
  return (
    <div className="mx-auto max-w-md space-y-7">
      <section className="space-y-3">
        <p className="text-sm font-semibold text-blue-300">{L("TrackTalkのコーチ機能")}</p>
        <p className="rounded-xl border border-blue-300/30 bg-blue-500/15 p-3 text-sm font-semibold text-blue-200">{L("コーチを募集しています。1回プラン（動画添削）は開始していて、申込みとチケットの購入ができます。継続プラン（1ヶ月の継続サポート）は準備中です。")}</p>
        <h1 className="text-2xl font-bold leading-relaxed">{L("あなたの陸上経験を、")}<br />{L("誰かの次の一歩に。")}</h1>
        <p className="text-sm leading-relaxed text-slate-200">{L("「フォームを見てほしい」「大会に向けて何をすればいい？」。そんな選手の悩みに、競技や指導で培った経験を生かして応える機能です。動画でのアドバイスから継続的な指導まで、自分が対応できる形で活動できます。")}</p>
      </section>

      <section className="space-y-4" aria-labelledby="coach-ways">
        <h2 id="coach-ways" className="text-lg font-bold">{L("関わり方は、大きく2つ")}</h2>
        <article className="space-y-3 rounded-2xl border border-blue-400/30 bg-blue-500/10 p-4">
          <h3 className="flex items-center gap-2 font-bold"><Shuffle className="size-5 text-blue-300" aria-hidden />{L("1回プラン：動画1本から支える")}</h3>
          <p className="text-sm leading-relaxed text-slate-200">{L("利用者は担当種目に合う受付中のコーチの中から1人を選び、種目と相談内容、動画を送ります。運営がコーチを割り振ることはありません。料金はどのコーチでも同じ1本500円で、コーチの報酬はその70％（350円）です。利用者がチケットで申し込んだ添削も、報酬は同じ350円です。一覧は毎日並び順が入れ替わり、依頼の件数や評価は表示しません。")}</p>
          <p className="text-sm leading-relaxed text-slate-200">{L("受付をオンにすると、申込みが対応中の一覧に届きます。届いてから24時間以内に良い点・直す点・おすすめのドリルを返してください。")}</p>
        </article>
        <article className="space-y-3 rounded-2xl border border-white/20 bg-white/10 p-4">
          <h3 className="flex items-center gap-2 font-bold"><UserRound className="size-5 text-blue-300" aria-hidden />{L("継続プラン：あなたに相談したい人を支える")}</h3>
          <p className="text-sm leading-relaxed text-slate-200">{L("継続プランは準備中です。開始後は、コーチが指導内容・料金・期間・定員を決めてプランを公開し、利用者はプロフィールや内容を見て申し込みます。申し込みをコーチが承諾した後、必要な支払い・同意を経て指導が始まります。")}</p>
          <ul className="space-y-3 text-sm text-slate-200">
            <li className="flex gap-2"><Trophy className="mt-0.5 size-4 shrink-0 text-blue-300" aria-hidden /><span><strong className="text-white">{L("1ヶ月プラン")}</strong>{L("：目標の大会に向けて、準備や調整をサポート。")}</span></li>
            <li className="flex gap-2"><MapPin className="mt-0.5 size-4 shrink-0 text-blue-300" aria-hidden /><span><strong className="text-white">{L("対面プラン")}</strong>{L("：場所や回数などをプランに明記し、現地で指導します。空き日カレンダーから希望日を受け取り、メッセージで相談して日時を提案・確定します。")}</span></li>
          </ul>
        </article>
      </section>

      <section className="space-y-3">
        <h2 className="text-lg font-bold">{L("自分のペースで、責任を持って")}</h2>
        <p className="text-sm leading-relaxed text-slate-200">{L("1回プランは受付のオン・オフで調整できます。期限を過ぎると新規受付は自動でオフになります。届いた依頼は期限まで対応できます。継続プランは対応できる内容と定員を自分で決めます。")}</p>
        <p className="text-sm leading-relaxed text-slate-200">{L("やり取りや支払いはTrackTalk上で行います。未成年の利用者の購入には、保護者の許可の確認など、所定の手続きがあります。")}</p>
      </section>

      <section className="space-y-3">
        <h2 className="text-lg font-bold">{L("報酬について")}</h2>
        <p className="text-sm leading-relaxed text-slate-200">{L("有料の依頼やプランで指導を提供すると、サービスの手数料を差し引いた報酬が記録されます。受け取り口座の登録、適用される手数料、報酬と送金の状況は、コーチ専用画面で確認できます。")}</p>
        <SupportPlanCoachTerms />
      </section>

      <section className="space-y-3">
        <h2 className="text-lg font-bold">{L("応募から活動まで")}</h2>
        <ol className="list-decimal space-y-2 pl-5 text-sm leading-relaxed text-slate-200">
          <li>{L("名前、担当種目、記録、成績や指導経験、顔写真と、SNSまたは所属チームのホームページのURLを入力します。URLは1〜3件必須です。")}</li>
          <li>{L("運営が内容を確認します。審査状況は応募画面で確認できます。")}</li>
          <li>{L("承認後は、コーチ画面から受付を始められます。承認状況や顔写真はコーチ画面で確認できます。")}</li>
        </ol>
      </section>
      <Link href="/coach/apply" replace className="flex min-h-12 items-center justify-between gap-3 rounded-xl bg-blue-600 px-4 py-3 text-sm font-semibold text-pure-white transition hover:bg-blue-500">{L("応募画面へ進む")}<ArrowRight className="size-4" aria-hidden />
      </Link>
    </div>
  );
}
