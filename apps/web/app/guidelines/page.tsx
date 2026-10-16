import type { Metadata } from "next";
import { Rule } from "@/components/legal-prose";

export const metadata: Metadata = {
  title: "コミュニティガイドライン",
  description: "TrackTalk のコミュニティガイドライン",
};

export default function GuidelinesPage() {
  return (
    <div className="mx-auto max-w-2xl space-y-6">
      <div className="space-y-2">
        <h1 className="text-2xl font-bold">コミュニティガイドライン</h1>
        <p className="text-sm leading-7 text-slate-300">
          議論のための場所です。記録やレースへの意見は歓迎します。人を傷つけること、権利を侵害すること、対面の集まりを悪用することは禁止です。違反した投稿は、通報を受けて、または運営の判断で非表示にすることがあります。
        </p>
      </div>

      <Rule title="選手と、他の利用者">
        <ul className="list-disc space-y-2 pl-5">
          <li>タイム、レース展開、戦術への批評はできます。</li>
          <li>侮辱、差別、脅迫、執拗な嫌がらせ、私生活の暴露は禁止です。</li>
          <li>根拠のない不正や私生活上の疑惑を、事実のように書き広げないでください。</li>
        </ul>
      </Rule>

      <Rule title="記録、映像、写真">
        <ul className="list-disc space-y-2 pl-5">
          <li>公式サイトや記録集からの無断の大量転載は禁止です。リンクと、議論に必要な短い引用にとどめてください。</li>
          <li>放送、配信、写真について、権利者の許諾がない複製をアップロードしないでください。</li>
          <li>人が写る画像や動画は、写った人の同意があるものだけにしてください。</li>
          <li>YouTube は、共有が許可されている動画の URL だけを投稿してください。</li>
          <li>Instagram は、公開されている投稿やリールの URL だけを投稿してください。</li>
        </ul>
      </Rule>

      <Rule title="個人を特定する情報">
        <ul className="list-disc space-y-2 pl-5">
          <li>電話番号、メールアドレス、住所、学校名、所属の連絡先、非公開の集合場所を投稿しないでください。</li>
          <li>他人のアカウントや成績と結びつけて、個人を特定できる書き方をしないでください。</li>
          <li>他の人のケガや病気など、健康に関する情報は書かないでください。自分のケガのことは、ケガ人の溜まり場に書けます。</li>
        </ul>
      </Rule>

      <Rule title="練習会と募集">
        <ul className="list-disc space-y-2 pl-5">
          <li>会場は、参加する人が自分で安全を判断できる場所にしてください。</li>
          <li>性的な目的の募集、金銭の要求、対面での迷惑行為は禁止です。</li>
          <li>未成年と分かる人を、二者だけで呼び出したり、個人的に連絡を求めたりしないでください。</li>
          <li>年齢層や男女比は募集の目安です。相手個人を特定する情報としては書かないでください。</li>
        </ul>
      </Rule>

      <Rule title="ケガ人の溜まり場">
        <ul className="list-disc space-y-2 pl-5">
          <li>書くのは、自分のケガのことだけにしてください。</li>
          <li>落ち込んでいる気持ちも、そのまま書いて大丈夫です。他の人の復帰の遅さや、ケガをしたことを責めないでください。</li>
          <li>治療法や薬を、効くと言い切ってすすめないでください。治療院、サプリ、器具などの宣伝は禁止です。</li>
          <li>紹介ページに載せる一言には、自分や他の人がわかる名前、学校名、所属を書かないでください。</li>
        </ul>
      </Rule>

      <Rule title="レビュー">
        <ul className="list-disc space-y-2 pl-5">
          <li>実際に使った、または試した商品について書いてください。</li>
          <li>メーカーや販売店の関係者が、そのことを隠して書くことは禁止です。</li>
        </ul>
      </Rule>

      <Rule title="なりすましと宣伝">
        <ul className="list-disc space-y-2 pl-5">
          <li>他の利用者、選手、大会主催者、運営者のなりすましは禁止です。</li>
          <li>陸上の話題と関係のない宣伝や、同じ内容の繰り返し投稿は禁止です。</li>
        </ul>
      </Rule>

      <Rule title="通報と削除の依頼">
        <p>
          投稿、コメント、紹介ページの一言には通報があります。運営は内容を確認し、ガイドラインまたは利用規約に反するものを非表示にできます。
        </p>
        <p>
          名誉、プライバシー、著作権などの侵害を知らせるときは、ログイン後のマイページ「お問い合わせ」から、対象の URL、侵害される権利、該当箇所を書いてください。運営は内容を確認し、非表示などの対応をします。
        </p>
      </Rule>
    </div>
  );
}
