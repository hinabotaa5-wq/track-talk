import type { Metadata } from "next";
import Link from "next/link";
import type { ReactNode } from "react";
import { operatorName, supportEmail } from "@/lib/operator";

// 特定商取引法に基づく表記(運営者が販売するフォーム添削・添削の追加相談)。チケットの新規販売は終了した。
// 弁護士Q&Aの回答をもとにしたたたき台で、文面は確定していない。Stripe で PayPay を有効にするには公開 URL が要るので、
// 有料の機能(COACHING_ENABLED)を出す前から、このページだけはログインなしで表示する。
// サポートプランはコーチが販売するので対象外。各プランの表記はコーチごとに出す。
// 価格は API の reviewPriceYen・ticketFaceYen(追加相談の料金)に合わせる。支払方法は API の checkoutPaymentMethods に合わせる。

export const metadata: Metadata = {
  title: "特定商取引法に基づく表記",
  description: "TrackTalk の運営者が販売するフォーム添削・追加相談についての特定商取引法に基づく表記。",
  robots: { index: false },
};

// 回答待ちで覆りうる欄。本番の文面が決まるまで外さない。
function Pending() {
  return <span className="ml-1 rounded bg-amber-500/20 px-1.5 py-0.5 text-xs text-amber-200">要確認</span>;
}

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
        <p className="rounded-lg border border-amber-400/40 bg-amber-500/15 p-3 text-sm leading-6 text-amber-100">
          この表記はたたき台です。内容は確定していません。
        </p>
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
          <p>フォーム添削 1本 1,500円（税込）。どのコーチを選んでも同じ料金です。</p>
          <p>フォーム添削の追加相談 1件 600円（税込）。</p>
        </Row>
        <Row term="商品代金以外の必要料金">インターネットの通信料は利用者の負担です。</Row>
        <Row term="支払方法">
          <p>クレジットカード（Apple Pay・Google Pay を含む）、PayPay。1回ごとのお支払いです。<Pending /></p>
        </Row>
        <Row term="支払時期">
          <p>申込みの時に支払います。PayPay は申込みの時に決済が済みます。</p>
          <p>クレジットカードは申込みの時に代金を確保し、添削・追加相談が返却された時に確定します。<Pending /></p>
        </Row>
        <Row term="提供時期">
          <p>フォーム添削は、申込みの時に利用者が選んだコーチが担当し、依頼した翌日の12時（日本時間）までの返却を目安にします。運営者がコーチを割り振ることはありません。追加相談は、コーチの受諾が48時間以内、受諾後の回答が72時間以内です。</p>
        </Row>
        <Row term="チケット">
          <p>チケットの新規販売は終了しました。購入済みのチケットの扱いは確定後に記載します。<Pending /></p>
        </Row>
        <Row term="キャンセル・返金">
          <p>期限までに添削・追加相談が返却されなかった場合や、選んだコーチが辞退した・30分以内に引き受けなかった場合は（別のコーチには回しません）、クレジットカードは代金の確保を取り消し、PayPay は全額を返金します（PayPay 残高に戻ります）。購入済みのチケットで支払った場合はチケットを戻します。</p>
          <p>それ以外の返金の条件（返却後の返金の可否など）は、提供を始める前に決めて記載します。<Pending /></p>
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
