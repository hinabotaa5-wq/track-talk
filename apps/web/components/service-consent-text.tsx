import Link from "next/link";

// 登録と、ログインから初めて利用する場合の同意を同じ文言にそろえる。
export function ServiceConsentText() {
  return (
    <span>
      <Link href="/terms" className="text-blue-400 underline">利用規約</Link>
      ・
      <Link href="/guidelines" className="text-blue-400 underline">コミュニティガイドライン</Link>
      ・
      <Link href="/privacy" className="text-blue-400 underline">プライバシーポリシー</Link>
      、および個人情報を海外の事業者に預けること（同ポリシー第7条）に同意する
    </span>
  );
}
