import type { Metadata } from "next";
import Link from "next/link";
import type { ReactNode } from "react";
import { operatorName, supportEmail } from "@/lib/operator";

// 有料機能の公開前から、運営者の表記だけをログインなしで表示する。
// サポートプランはコーチが販売するため、各プランの表記を参照する。
// API の reviewPriceYen / ticketFaceYen と一致することを page.test.tsx で検証する。
const reviewPriceYen = 1500;
const consultationPriceYen = 600;
const yen = (amount: number) => amount.toLocaleString("ja-JP");

export const metadata: Metadata = {
  title: "特定商取引法に基づく表記",
  description: "TrackTalk の運営者が販売するフォーム添削・追加相談についての特定商取引法に基づく表記。",
  robots: { index: false },
};

function Row({ term, children }: { term: string; children: ReactNode }) {
  return (
    <div className="grid gap-1 border-t border-white/15 py-3 sm:grid-cols-[11rem_1fr] sm:gap-4">
      <dt className="text-sm font-semibold">{term}</dt>
      <dd className="space-y-1 text-sm leading-7 text-slate-300">{children}</dd>
    </div>
  );
}

export default function CommercePage() {
  return (
    <div className="mx-auto max-w-2xl space-y-6 [overflow-wrap:anywhere]">
      <div className="space-y-2">
        <h1 className="text-2xl font-bold">特定商取引法に基づく表記</h1>
        <p className="text-sm leading-7 text-slate-300">
          運営者が販売する「フォーム添削」「フォーム添削の追加相談」についての表記です。サポートプラン（大会サポート・対面指導などを含むコーチのプラン）はコーチが販売者になるため、各プランのページにあるコーチの表記を確認してください。
        </p>
      </div>
      <dl>
        <Row term="販売事業者">{operatorName}（個人）</Row>
        <Row term="運営統括責任者">{operatorName}</Row>
        <Row term="所在地">
          <p>請求があれば遅滞なく開示します。</p>
        </Row>
        <Row term="電話番号">
          <p>請求があれば遅滞なく開示します。</p>
        </Row>
        <Row term="開示の請求">
          <p>
            ログインしている場合はマイページの<Link href="/me/contact?category=disclosure" className="text-blue-300 underline">お問い合わせ</Link>で種類「特商法表記の開示請求」を選んでください。ログインしていない場合は、件名を「特商法表記の開示請求」として<a href={`mailto:${supportEmail}`} className="text-blue-300 underline">{supportEmail}</a>へ送ってください。
          </p>
        </Row>
        <Row term="メールアドレス">
          <a href={`mailto:${supportEmail}`} className="text-blue-300 underline">{supportEmail}</a>
        </Row>
        <Row term="販売価格">
          <p>フォーム添削 1本 {yen(reviewPriceYen)}円（税込）。どのコーチを選んでも同じ料金です。</p>
          <p>フォーム添削の追加相談 1件 {yen(consultationPriceYen)}円（税込）。</p>
        </Row>
        <Row term="商品代金以外の必要料金">インターネットの通信料は利用者の負担です。</Row>
        <Row term="支払方法">
          <p>クレジットカード（Apple Pay・Google Pay を含む）、PayPay。1回ごとのお支払いです。</p>
        </Row>
        <Row term="支払時期">
          <p>フォーム添削は、PayPay の場合は申込みの時に決済が済みます。</p>
          <p>フォーム添削のクレジットカード払いは、申込みの時に代金を確保し、添削が返却された時に確定します。</p>
          <p>フォーム添削の追加相談は、クレジットカード・PayPay ともに申込みの時に決済が済みます。</p>
        </Row>
        <Row term="提供時期">
          <p>フォーム添削は、申込みの時に利用者が選んだコーチが担当し、依頼した翌日の12時（日本時間）までの返却を目安にします。運営者がコーチを割り振ることはありません。追加相談は、決済完了後48時間以内にコーチが受諾し、受諾後72時間以内に回答します。</p>
        </Row>
        <Row term="キャンセル・返金">
          <p>フォーム添削は、返却期限（依頼した翌日の12時・日本時間）までに添削が返却されなかった場合、選んだコーチが辞退した場合、または打診から30分以内（返却期限が先に来る場合はその期限まで）に引き受けなかった場合、申込みは期限切れとなります。別のコーチには回しません。クレジットカードは代金の確保を取り消し、PayPay は全額を返金します。</p>
          <p>終了した依頼とは独立した新しい申込みとして、別のコーチに改めて申し込むことができます。</p>
          <p>フォーム添削の追加相談は、コーチが辞退した場合、決済完了後48時間以内に受諾しなかった場合、または受諾後72時間以内に回答しなかった場合、申込みを終了し、クレジットカード・PayPay ともに全額を返金します。</p>
        </Row>
        <Row term="動作環境">最新の Safari・Chrome・Edge・Firefox。動画の送信には安定した通信環境が必要です。</Row>
      </dl>
      <nav className="flex flex-wrap gap-4 text-sm text-blue-300 underline">
        <Link href="/terms">利用規約</Link>
        <Link href="/privacy">プライバシーポリシー</Link>
        <Link href="/contact">運営者・お問い合わせ</Link>
      </nav>
    </div>
  );
}
