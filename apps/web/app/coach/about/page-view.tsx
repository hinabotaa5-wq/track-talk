"use client";

import { L } from "@/lib/i18n";
import Link from "next/link";
import { ArrowRight, Shuffle } from "lucide-react";


export function CoachAboutView() {
  return (
    <div className="mx-auto max-w-md space-y-7">
      <section className="space-y-3">
        <p className="text-sm font-semibold text-blue-300">{L("TrackTalkのコーチ機能")}</p>
        <p className="rounded-xl border border-blue-300/30 bg-blue-500/15 p-3 text-sm font-semibold text-blue-200">{L("コーチを募集しています。スタンダードは開始していて、申込みとチケットの購入ができます。")}</p>
        <h1 className="text-2xl font-bold leading-relaxed">{L("あなたの陸上経験を、")}<br />{L("誰かの次の一歩に。")}</h1>
        <p className="text-sm leading-relaxed text-slate-200">{L("「フォームを見てほしい」「大会に向けて何をすればいい？」。そんな選手の悩みに、競技や指導で培った経験を生かして応える機能です。動画や文章でのアドバイスを通じて、自分が対応できる形で活動できます。")}</p>
      </section>

      <section className="space-y-4" aria-labelledby="coach-ways">
        <h2 id="coach-ways" className="text-lg font-bold">{L("関わり方")}</h2>
        <article className="space-y-3 rounded-2xl border border-blue-400/30 bg-blue-500/10 p-4">
          <h3 className="flex items-center gap-2 font-bold"><Shuffle className="size-5 text-blue-300" aria-hidden />{L("スタンダード")}</h3>
          <p className="text-sm leading-relaxed text-slate-200">{L("利用者は担当種目に合う受付中のコーチの中から1人を選び、種目と相談内容を送り、必要に応じて動画を添えます。運営がコーチを割り振ることはありません。料金はコーチごとに異なります（コーチが自分で申請し、運営の承認の後に切り替わります）。コーチの報酬は、その依頼の料金の70％です。利用者がチケットで申し込んだスタンダードの報酬は700円です。一覧は毎日並び順が入れ替わり、依頼の件数や評価は表示しません。")}</p>
          <p className="text-sm leading-relaxed text-slate-200">{L("受付をオンにすると、申込みが対応中の一覧に届きます。届いてから24時間以内に、全コーチ共通の良い点・直す点・おすすめのドリル・コーチなら明日から何をするかと、申込みの時に決まった追加の項目のすべてに回答して返してください。")}</p>
          <p className="text-sm leading-relaxed text-slate-200">{L("添削の追加の項目と、コーチのページに載せる見本動画は、コーチ画面から申請できます。運営の承認の後、料金と同じく対応中の依頼がすべて終わってから切り替わり、その後の新しい申込みから適用されます。")}</p>
        </article>
      </section>

      <section className="space-y-3">
        <h2 className="text-lg font-bold">{L("自分のペースで、責任を持って")}</h2>
        <p className="text-sm leading-relaxed text-slate-200">{L("スタンダードは受付のオン・オフで調整できます。期限を過ぎると新規受付は自動でオフになります。届いた依頼は期限まで対応できます。")}</p>
        <p className="text-sm leading-relaxed text-slate-200">{L("やり取りや支払いはTrackTalk上で行います。未成年の利用者の購入には、保護者の許可の確認など、所定の手続きがあります。")}</p>
      </section>

      <section className="space-y-3">
        <h2 className="text-lg font-bold">{L("報酬について")}</h2>
        <p className="text-sm leading-relaxed text-slate-200">{L("有料の依頼やプランで指導を提供すると、サービスの手数料を差し引いた報酬が記録されます。受け取り口座の登録、適用される手数料、報酬と送金の状況は、コーチ専用画面で確認できます。")}</p>
      </section>

      <section className="space-y-3">
        <h2 className="text-lg font-bold">{L("応募から活動まで")}</h2>
        <ol className="list-decimal space-y-2 pl-5 text-sm leading-relaxed text-slate-200">
          <li>{L("名前、担当種目、記録、成績や指導経験、顔写真と、SNSのアカウント名またはSNS・所属チームのホームページのURLを入力します。SNS・ホームページ欄は1〜3件必須（各300文字以内）です。")}</li>
          <li>{L("運営が内容を確認します。審査状況は応募画面で確認できます。")}</li>
          <li>{L("承認後は、コーチ画面から受付を始められます。承認状況や顔写真はコーチ画面で確認できます。")}</li>
        </ol>
      </section>
      <Link href="/coach/apply" replace className="flex min-h-12 items-center justify-between gap-3 rounded-xl bg-blue-600 px-4 py-3 text-sm font-semibold text-pure-white transition hover:bg-blue-500">{L("応募画面へ進む")}<ArrowRight className="size-4" aria-hidden />
      </Link>
    </div>
  );
}
