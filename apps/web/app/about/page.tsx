export default function AboutPage() {
  return (
    <div className="mx-auto max-w-2xl space-y-10">
      <section className="space-y-4">
        <h1 className="text-2xl font-bold">TRACK TALKについて</h1>
        <p className="text-zinc-700">
          TRACK TALK は、陸上競技の大会結果やレース展開、記録についてファン同士が議論するためのコミュニティです。公式記録の網羅ではなく、会話を主軸にしています。
        </p>
      </section>
      <section id="guidelines" className="scroll-mt-20 space-y-4">
        <h2 className="text-2xl font-bold">コミュニティガイドライン</h2>
        <ul className="list-disc space-y-2 pl-5 text-zinc-700">
          <li>選手・他の利用者への侮辱やハラスメントは禁止です。</li>
          <li>公式記録サイトからの無断の大量転載はしないでください。</li>
          <li>個人情報や秘密情報を投稿しないでください。</li>
          <li>不適切な投稿は通報できます。運営が非表示にする場合があります。</li>
        </ul>
      </section>
      <section id="terms" className="scroll-mt-20 space-y-4">
        <h2 className="text-2xl font-bold">利用規約</h2>
        <p className="text-sm text-zinc-500">公開前に内容を弁護士またはテンプレートで最終確認してください。</p>
        <ol className="list-decimal space-y-2 pl-5 text-zinc-700">
          <li>本サービスは陸上競技に関する議論の場を提供します。</li>
          <li>投稿の著作権は投稿者に帰属し、運営はサービス運営に必要な範囲で利用できます。</li>
          <li>法令違反、権利侵害、迷惑行為を禁止します。</li>
          <li>運営は投稿の非表示・アカウント対応を行うことがあります。</li>
          <li>個人開発のため、予告なく停止・変更する場合があります。</li>
        </ol>
      </section>
      <section id="privacy" className="scroll-mt-20 space-y-4">
        <h2 className="text-2xl font-bold">プライバシーポリシー</h2>
        <p className="text-sm text-zinc-500">公開前に内容を最終確認してください。</p>
        <ul className="list-disc space-y-2 pl-5 text-zinc-700">
          <li>取得する情報: メールアドレス、投稿・コメント、通報内容、お問い合わせ内容。名前や自己紹介は取得しません。</li>
          <li>利用目的: アカウント管理、コミュニティ運営、不正利用への対応。</li>
          <li>認証は Supabase Auth、データベースは PostgreSQL（Supabase）を利用します。</li>
          <li>フロントエンドにデータベース接続情報や管理者用秘密鍵は置きません。</li>
          <li>問い合わせ: ログイン後、マイページの「お問い合わせ」から運営へ送れます。</li>
        </ul>
      </section>
    </div>
  );
}
