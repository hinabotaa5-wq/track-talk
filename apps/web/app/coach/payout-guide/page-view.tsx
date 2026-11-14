"use client";

import { L } from "@/lib/i18n";
import Link from "next/link";


const SITE_URL = "https://track-talk.netlify.app";
// Stripe の画面に日本語のまま貼る文なので、英語表示でも訳さない。
const PRODUCT_DESCRIPTION = "陸上競技の1回プランのサービスです。"; // i18n-data

// 手順の図は Stripe の画面写真ではなく、個人情報が写らない範囲の模式図。
// Stripe 側の文言は変わることがあるため、選ぶ内容を文字で明示する。
function Figure({ label, children }: { label: string; children: React.ReactNode }) {
  return (
    <figure className="overflow-hidden rounded-xl border border-white/20 bg-white/5" aria-label={label}>
      <div className="border-b border-white/10 px-3 py-1.5 text-xs text-slate-300">{label}</div>
      <div className="space-y-2 p-3">{children}</div>
    </figure>
  );
}

function CopyBlock({ label, value }: { label: string; value: string }) {
  return (
    <div className="space-y-1">
      <p className="text-xs font-semibold text-slate-200">{label}</p>
      <p className="rounded-lg border border-white/25 bg-black/30 p-2 text-sm font-mono break-all text-white select-all">
        {value}
      </p>
      <p className="text-xs text-slate-400">{L("長押し・ダブルクリックでコピーできます。")}</p>
    </div>
  );
}

function Step({ n, title, children }: { n: number; title: string; children: React.ReactNode }) {
  return (
    <section className="space-y-2" aria-label={L("手順{n}：{title}", { n, title })}>
      <h2 className="text-base font-bold">
        <span className="mr-2 inline-flex size-6 items-center justify-center rounded-full bg-blue-600 text-sm text-white">{n}</span>
        {title}
      </h2>
      <div className="space-y-2 text-sm leading-7 text-slate-200">{children}</div>
    </section>
  );
}

export function CoachPayoutGuideView() {
  return (
    <div className="mx-auto max-w-md space-y-7">
      <div className="space-y-2">
        <p className="text-sm font-semibold text-blue-300">{L("コーチ向け・受け取り口座の登録")}</p>
        <h1 className="text-2xl font-bold leading-relaxed">{L("口座登録の手順")}</h1>
        <p className="text-sm leading-relaxed text-slate-200">{L("報酬を受け取るための口座登録（Stripe）の手順です。初めての方でも、このページの順番どおりに進めれば登録できます。10分ほどで終わります。")}</p>
        <p className="text-xs leading-relaxed text-slate-400">{L("図は画面写真ではなく内容を示す模式図です。Stripe 側のボタンの文言は変わることがありますが、選ぶ内容はこのページのとおりです。")}</p>
      </div>

      <Step n={1} title={L("コーチ画面のボタンから始める")}>
        <p>{L("コーチ画面の「報酬」にある黄色い枠の")}<button type="button" className="mx-1 rounded-md bg-blue-600 px-2 py-0.5 text-sm font-semibold text-white">{L("受け取り口座を登録する")}</button>{L("を押します。登録の途中で止まっている場合は「登録を続ける」と表示されます。")}</p>
        <Figure label={L("模式図：コーチ画面「報酬」の未登録時の表示")}>
          <div className="rounded-lg border border-amber-400/50 bg-amber-500/15 p-2 text-xs">
            <p className="font-bold text-white">{L("● 受け取り口座： 未登録")}</p>
            <p className="mt-1 rounded-md bg-blue-600 px-2 py-1.5 text-center font-semibold text-white">{L("受け取り口座を登録する")}</p>
          </div>
        </Figure>
        <p>{L("押すと Stripe の登録画面が開きます。以降は Stripe の画面で入力します。")}</p>
      </Step>

      <Step n={2} title={L("事業形態は「個人事業主」を選ぶ")}>
        <p>{L("事業形態の選択では")}<strong className="text-white">{L("「個人事業主」")}</strong>{L("を選んで次へ進みます。法人・非営利組織の選択肢は選びません。")}</p>
        <Figure label={L("模式図：事業形態の選択")}>
          <ul className="space-y-1 text-xs">
            <li className="rounded-md border border-blue-400 bg-blue-500/25 px-2 py-1.5 font-bold text-white">{L("● 個人事業主 ← これを選ぶ")}</li>
            <li className="rounded-md border border-white/20 px-2 py-1.5 text-slate-300">{L("○ 法人")}</li>
            <li className="rounded-md border border-white/20 px-2 py-1.5 text-slate-300">{L("○ 非営利組織")}</li>
          </ul>
        </Figure>
      </Step>

      <Step n={3} title={L("ビジネスの詳細をそのまま入力する")}>
        <p>{L("「ビジネスの詳細」の画面では、下の3つをそのまま入力します。コピーして使ってください。")}</p>
        <div className="space-y-3 rounded-xl border border-white/15 bg-white/5 p-3">
          <div className="space-y-1">
            <p className="text-xs font-semibold text-slate-200">{L("業種")}</p>
            <p className="rounded-lg border border-white/25 bg-black/30 p-2 text-sm font-bold text-white select-all">{L("教育 ＞ その他の教育サービス")}</p>
            <p className="text-xs text-slate-400">{L("候補が多いので、検索欄に「教育」と入れて絞り、「その他の教育サービス」を選びます。")}</p>
          </div>
          <CopyBlock label={L("ウェブサイト（このまま入力）")} value={SITE_URL} />
          <CopyBlock label={L("商品の説明（このまま入力）")} value={PRODUCT_DESCRIPTION} />
        </div>
        <Figure label={L("模式図：ビジネスの詳細の入力欄")}>
          <ul className="space-y-1 text-xs text-slate-200">
            <li className="rounded-md border border-white/20 px-2 py-1.5">{L("業種： 教育 ＞ その他の教育サービス")}</li>
            <li className="rounded-md border border-white/20 px-2 py-1.5 font-mono break-all">{SITE_URL}</li>
            <li className="rounded-md border border-white/20 px-2 py-1.5">{PRODUCT_DESCRIPTION}</li>
          </ul>
        </Figure>
      </Step>

      <Step n={4} title={L("本人情報・本人確認・口座情報を入力する")}>
        <p>{L("氏名・住所・生年月日・本人確認書類・銀行口座を、画面の案内に沿って入力します。このページには個人情報の画面の図は載せていません。")}</p>
        <ul className="list-disc space-y-1 pl-5 text-sm leading-relaxed">
          <li>{L("本人確認書類は、有効期限内のものを用意してください。")}</li>
          <li>{L("口座名義は本人名義のものを使います。")}</li>
          <li>{L("入力中に Stripe から追加の確認を求められることがあります。その場合は画面の指示に従ってください。")}</li>
        </ul>
      </Step>

      <Step n={5} title={L("TrackTalk に戻って「登録済み」を確認する")}>
        <p>{L("登録が終わったら TrackTalk に戻り、コーチ画面の「報酬」が緑色の表示になっていることを確認します。")}</p>
        <Figure label={L("模式図：登録が済んだときの表示")}>
          <div className="rounded-lg border border-emerald-400/50 bg-emerald-500/15 p-2 text-xs">
            <p className="font-bold text-white">{L("● 受け取り口座： 登録済み")}</p>
            <p className="mt-1 text-slate-200">{L("報酬は登録済みの口座に振り込まれます。")}</p>
          </div>
        </Figure>
        <p>{L("「登録の途中・確認中」と出たままのときは、Stripe 側で確認中か、入力の続きがあります。「登録を続ける」ボタンから続きを入力してください。")}</p>
      </Step>

      <Step n={6} title={L("戻った後に画面が真っ白になったら開き直す")}>
        <p>{L("Stripe から戻った直後に、フィードや大会の画面が真っ白になったり、ヘッダーが出なかったりすることがあります。そのときは")}<strong className="text-white">{L("アプリを完全に閉じて、もう一度開いてください")}</strong>{L("。登録自体は終わっていることがほとんどで、開き直すと正常に戻ります。")}</p>
        <p>{L("開き直しても「未登録」のままのときは、コーチ画面のボタンから登録をやり直してください。うまくいかないときは")}<Link href="/contact" className="text-blue-300 underline">{L("お問い合わせ")}</Link>{L("から運営に連絡してください。")}</p>
      </Step>

      <section className="space-y-2 border-t border-white/15 pt-4">
        <h2 className="text-base font-bold">{L("困ったとき")}</h2>
        <p className="text-sm leading-relaxed text-slate-200">{L("業種が見つからない、入力内容で迷うなど、このページで解決しないときは")}<Link href="/contact" className="text-blue-300 underline">{L("お問い合わせ")}</Link>{L("から運営に連絡してください。その際「口座登録の手順の手順◯で止まっている」と書くとスムーズです。")}</p>
        <p className="text-sm leading-relaxed text-slate-200">
          <Link href="/coach" className="text-blue-300 underline">{L("コーチ画面に戻る")}</Link>
        </p>
      </section>
    </div>
  );
}
