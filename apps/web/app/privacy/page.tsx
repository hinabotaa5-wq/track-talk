export default function PrivacyPage() {
  return (
    <div className="space-y-4">
      <h1 className="text-2xl font-bold">プライバシーポリシー</h1>
      <p className="text-sm text-zinc-500">公開前に内容を最終確認してください。</p>
      <ul className="list-disc space-y-2 pl-5 text-zinc-700">
        <li>取得する情報: メールアドレス、ニックネーム、投稿・コメント、通報内容。</li>
        <li>利用目的: アカウント管理、コミュニティ運営、不正利用への対応。</li>
        <li>認証は Supabase Auth、データベースは PostgreSQL（Supabase）を利用します。</li>
        <li>フロントエンドにデータベース接続情報や管理者用秘密鍵は置きません。</li>
        <li>問い合わせ: 運営者が公開時に記載するメールアドレスへ。</li>
      </ul>
    </div>
  );
}
