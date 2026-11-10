"use client";

import { L } from "@/lib/i18n";
import Link from "next/link";
import { operatorName, supportEmail } from "@/lib/operator";


export function ContactView() {
  return <div className="mx-auto max-w-2xl space-y-6 [overflow-wrap:anywhere]">
    <h1 className="text-2xl font-bold">{L("運営者・お問い合わせ")}</h1>
    <dl className="space-y-4 text-sm">
      <div><dt className="font-semibold">{L("サービス名")}</dt><dd className="mt-1 text-slate-300">TrackTalk</dd></div>
      <div><dt className="font-semibold">{L("運営者")}</dt><dd className="mt-1 text-slate-300">{operatorName}</dd></div>
      <div><dt className="font-semibold">{L("メールアドレス")}</dt><dd className="mt-1"><a href={`mailto:${supportEmail}`} className="text-blue-300 underline">{supportEmail}</a></dd></div>
    </dl>
    <p className="text-sm leading-7 text-slate-300">{L("ログインできない場合や、退会後のお問い合わせもメールで受け付けます。支払い・返金については、分かる範囲で購入日時や購入番号をお知らせください。パスワード、カード番号、保護者の同意ページのリンクは送らないでください。")}</p>
    <p className="text-sm leading-7 text-slate-300">{L("ログインしている場合は、マイページの")}<Link href="/me/contact" className="text-blue-300 underline">{L("お問い合わせ")}</Link>{L("も利用できます。")}</p>
    <section className="space-y-2">
      <h2 className="text-base font-semibold">{L("アカウントを乗っ取られたかもしれないとき")}</h2>
      <p className="text-sm leading-7 text-slate-300">{L("覚えのないログインや投稿、メールアドレスやパスワードの変更に気づいたときは、件名を「アカウント乗っ取りの申告」として、上のメールアドレスへお知らせください。できるだけ、ご登録のメールアドレスから送ってください。ユーザー名、気づいた日時、最後に普段どおり使えた日時を書いていただけると調査が早くなります。")}</p>
      <p className="text-sm leading-7 text-slate-300">{L("運営からパスワードや認証コードをお尋ねすることはありません。メールにも書かないでください。ご本人確認は、ご登録のメールアドレスでのやり取りで行います。")}</p>
    </section>
    <nav className="flex flex-wrap gap-4 text-sm text-blue-300 underline"><Link href="/terms">{L("利用規約")}</Link><Link href="/privacy">{L("プライバシーポリシー")}</Link><Link href="/legal/commerce">{L("特定商取引法に基づく表記")}</Link></nav>
  </div>;
}
