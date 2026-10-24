import type { Metadata } from "next";
import Link from "next/link";
import { Rule } from "@/components/legal-prose";
import { CoachApplyLink } from "@/components/coach-apply-link";

export const metadata: Metadata = { title: "利用ガイド", description: "TrackTalkの投稿・大会・メモ・ケガの使い方とコーチ募集の案内。" };

export default function GuidePage() {
  return (
    <div className="mx-auto max-w-2xl space-y-7">
      <div className="space-y-2">
        <h1 className="text-2xl font-bold">TrackTalkの利用ガイド</h1>
        <p className="text-sm leading-relaxed text-slate-300">陸上の話をしたいとき、大会を探すとき、練習の気づきを残したいときに。下の5つのタブから使えます。</p>
      </div>
      <Rule title="ホーム：投稿を見て、話す">
        <p>「新しい」「人気」「募集」「レビュー」を切り替えて探します。投稿一覧は左右にスワイプでき、虫眼鏡からキーワードで検索できます。</p>
        <p>写真や動画の下にある文が投稿の内容です。カードを押すと全文と動画、コメントが開き、コメントのカードを押すとその会話に入れます。自分のコメントを長押しすると、編集や削除などの操作を選べます。</p>
        <p>ホームの「＋」から文章・動画URL・自分の動画・クラブチームや練習相手の募集を投稿できます。投稿・いいね・保存・コメントにはログインが必要です。</p>
      </Rule>
      <Rule title="大会：日程と実況を探す">
        <p>月や地域を選んで大会を探します。詳しい開催情報は主催者の案内を確認してください。今日開催中の大会があるときは、ヘッダーのトロフィーから実況スレッドを開けます。</p>
      </Rule>
      <Rule title="メモ：自分の感覚と記録を残す">
        <p>練習で感じたことは「感覚」に、競技会の結果は「記録」に残します。どちらも本人だけが見られます。感覚メモを書くとビンに玉がたまり、練習の積み重ねを振り返れます。</p>
      </Rule>
      <Rule title="コーチ：現在はコーチ募集中">
        <p>ランダムプランと指名プランは準備中です。いまは指導の申込み・チケットの購入を受け付けていません。</p>
        <p>コーチとして活動したい方は「コーチ専用画面」へ進んでください。未ログインの場合はログイン画面が開き、その下の「コーチに応募する」から応募できます。ログイン済みの場合は応募の案内や審査状況を確認できます。</p>
        <p>応募には名前・担当種目・記録・成績や指導経験、顔写真、SNSまたは所属チームのホームページのURLが必要です。リンクは1件以上、最大3件です。承認後も、公開までは準備中の画面になります。</p>
        <CoachApplyLink />
      </Rule>
      <Rule title="ケガ：経験や復帰までの経過を共有する">
        <p>自分のケガに合うグループに入り、段階に合わせて気持ちや経過を共有できます。グループの中の投稿は参加者だけに表示され、ホームには出ません。公開される集計は個人がわからない形です。</p>
        <p>投稿や集計は診断・治療の代わりにはなりません。症状や復帰の判断は医療の専門家に相談してください。</p>
      </Rule>
      <Rule title="通知・マイページ・困ったとき">
        <p>ヘッダーのベルから通知、人のアイコンからマイページを開けます。自分の投稿や保存した投稿、プロフィールや通知の設定はマイページにあります。最後に開いたメインタブは、同じブラウザで次回起動時に復元します。</p>
        <p>不具合や要望はヘッダーの箱アイコンの「なんでも書いてね」へ。返信が必要な連絡はマイページの「お問い合わせ」から送ってください。不適切な投稿には通報を使えます。</p>
        <p><Link href="/guidelines" className="text-blue-300 underline">コミュニティガイドライン</Link>と<Link href="/terms" className="text-blue-300 underline">利用規約</Link>、<Link href="/privacy" className="text-blue-300 underline">プライバシーポリシー</Link>も確認してください。</p>
      </Rule>
    </div>
  );
}
