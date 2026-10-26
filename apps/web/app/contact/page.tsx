import type { Metadata } from "next";
import Link from "next/link";
import { operatorName, supportEmail } from "@/lib/operator";

export const metadata: Metadata = {
  title: "運営者・お問い合わせ",
  description: "TrackTalkの運営者と、ログインせずに利用できるお問い合わせ先。",
};

export default function ContactPage() {
  return <div className="mx-auto max-w-2xl space-y-6 [overflow-wrap:anywhere]">
    <h1 className="text-2xl font-bold">運営者・お問い合わせ</h1>
    <dl className="space-y-4 text-sm">
      <div><dt className="font-semibold">サービス名</dt><dd className="mt-1 text-slate-300">TrackTalk</dd></div>
      <div><dt className="font-semibold">運営者</dt><dd className="mt-1 text-slate-300">{operatorName}</dd></div>
      <div><dt className="font-semibold">メールアドレス</dt><dd className="mt-1"><a href={`mailto:${supportEmail}`} className="text-blue-300 underline">{supportEmail}</a></dd></div>
    </dl>
    <p className="text-sm leading-7 text-slate-300">ログインできない場合や、退会後のお問い合わせもメールで受け付けます。支払い・返金については、分かる範囲で購入日時や購入番号をお知らせください。パスワード、カード番号、保護者の同意ページのリンクは送らないでください。</p>
    <p className="text-sm leading-7 text-slate-300">ログインしている場合は、マイページの<Link href="/me/contact" className="text-blue-300 underline">お問い合わせ</Link>も利用できます。</p>
    <nav className="flex flex-wrap gap-4 text-sm text-blue-300 underline"><Link href="/terms">利用規約</Link><Link href="/privacy">プライバシーポリシー</Link></nav>
  </div>;
}
