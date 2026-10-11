export default function TermsPage() {
  return (
    <div className="space-y-4">
      <h1 className="text-2xl font-bold">利用規約</h1>
      <p className="text-sm text-zinc-500">公開前に内容を弁護士またはテンプレートで最終確認してください。</p>
      <ol className="list-decimal space-y-2 pl-5 text-zinc-700">
        <li>本サービスは陸上競技に関する議論の場を提供します。</li>
        <li>投稿の著作権は投稿者に帰属し、運営はサービス運営に必要な範囲で利用できます。</li>
        <li>法令違反、権利侵害、迷惑行為を禁止します。</li>
        <li>運営は投稿の非表示・アカウント対応を行うことがあります。</li>
        <li>個人開発のため、予告なく停止・変更する場合があります。</li>
      </ol>
    </div>
  );
}
