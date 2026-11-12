import type { Metadata } from "next";
import Link from "next/link";
import type { ReactNode } from "react";
import { operatorName, supportEmail } from "@/lib/operator";
import { JapaneseOfficialNote } from "@/components/japanese-official-note";

// 有料機能の公開前から、運営者の表記だけをログインなしで表示する。
// サポートプランはコーチが販売するため、各プランの表記を参照する。
// 料金は lib/review-pricing.ts（API の reviewPriceYen とチケットの商品）から取り、page.test.tsx で API 側と突き合わせる（本体 #569）。
import { reviewPriceYen, ticketPackPriceYen, ticketPackQuantity, ticketValidityMonths } from "@/lib/review-pricing";
const yen = (amount: number) => amount.toLocaleString("ja-JP");

export const metadata: Metadata = {
  title: "特定商取引法に基づく表記",
  description: "TrackTalk の運営者が販売するフォーム添削・チケットについての特定商取引法に基づく表記。",
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
        <JapaneseOfficialNote />
        <p className="text-sm leading-7 text-slate-300">
          運営者が販売する「フォーム添削」「フォーム添削のチケット」についての表記です。サポートプラン（大会サポート・対面指導などを含むコーチのプラン）はコーチが販売者になるため、各プランのページにあるコーチの表記を確認してください。
        </p>
        <p className="text-sm leading-7 text-slate-300">
          運営者が販売する「海外コーチ 1ヶ月コーチング（パイロット）」についての表記です。
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
          <p>フォーム添削のチケット {ticketPackQuantity}枚 {yen(ticketPackPriceYen)}円（税込）。{ticketPackQuantity}枚をまとめて先に支払います。1枚でフォーム添削1本を申し込めます。有効期限は決済完了の日時から{ticketValidityMonths}か月で、過ぎると使えなくなります（延長はしません）。購入した本人だけが使え、譲渡はできません。</p>
          <p>海外コーチ 1ヶ月コーチング（パイロット）は 1ヶ月コーチング 22,000円（税込）。</p>
        </Row>
        <Row term="商品代金以外の必要料金">
          <p>インターネットの通信料は利用者の負担です。</p>
          <p>海外コーチ 1ヶ月コーチング（パイロット）もインターネットの通信料は利用者の負担です。</p>
        </Row>
        <Row term="支払方法">
          <p>クレジットカード（Apple Pay・Google Pay を含む）。1回ごとのお支払いです。</p>
          <p>海外コーチ 1ヶ月コーチング（パイロット）はクレジットカード（決済リンク経由。1回ごとのお支払いです）。</p>
        </Row>
        <Row term="支払時期">
          <p>フォーム添削のクレジットカード払いは、申込みの時に代金を確保し、添削が返却された時に確定します。</p>
          <p>フォーム添削のチケットは、購入の時に{ticketPackQuantity}枚分の代金をまとめて決済します。</p>
          <p>海外コーチ 1ヶ月コーチング（パイロット）はお申込み（決済リンクでのお支払い）の時に決済が完了します。</p>
        </Row>
        <Row term="提供時期">
          <p>フォーム添削は、申込みの時に利用者が選んだコーチが担当し、申込みから24時間後（返却期限。夜間も含めて数えます）までに返却します。ただし、担当のコーチが内容確認のために依頼を差し戻し（返却の前に1つの依頼につき1回まで）、利用者が再提出した場合は、再提出から24時間後が新しい返却期限です。運営者がコーチを割り振ることはありません。</p>
          <p>フォーム添削のチケットは、決済の完了を確認した時から使えます。</p>
          <p>海外コーチ 1ヶ月コーチング（パイロット）はお支払いの確認後、案内した期間（1ヶ月間）について指導を提供します。</p>
        </Row>
        <Row term="キャンセル・返金">
          <p>フォーム添削は、返却期限（申込みから24時間後）までに添削が返却されなかった場合、選んだコーチが辞退した場合（引き受けた後、返却の前の辞退を含みます）、またはお支払いの完了を受けて、利用者が選んだコーチへの打診をシステムが作成した時刻から90分以内（日本時間の23時から翌7時までは数えません）に引き受けなかった場合、申込みは期限切れとなります。別のコーチには回しません。クレジットカードは代金の確保を取り消し、チケットで申し込んだ場合は使った1枚を戻します。</p>
          <p>担当のコーチが内容確認のために依頼を差し戻した場合、差し戻しの間は代金の確保の取消しや返金はしません。差し戻しから24時間以内（夜間も含めて数えます）に再提出がなかった場合、申込みは終了し、別のコーチには回しません。クレジットカードは代金の確保を取り消し、チケットで申し込んだ場合は使った1枚を戻します。</p>
          <p>終了した依頼とは独立した新しい申込みとして、別のコーチに改めて申し込むことができます。</p>
          <p>フォーム添削のチケットは、購入後の払い戻しを原則として行いません。有効期限を過ぎて失効したチケットも払い戻しません。ただし、運営者がフォーム添削の提供を終了した場合、未成年者が法定代理人の許可なく購入したとの申立てにより購入を取り消した場合、運営者の責めに帰すべき事由によってフォーム添削を受けられなかった場合は、未使用のチケットを払い戻します（購入額を枚数で割った額に未使用の枚数を掛けた額。1円未満は切り捨てます）。詳しくは利用規約第6条のとおりです。</p>
          <p>海外コーチ 1ヶ月コーチング（パイロット）は途中で解約する場合、その月の料金の返金はなく、期間は月末までとなります。コーチ側の事情で指導が提供できなかった部分については、未提供分を返金します。</p>
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
