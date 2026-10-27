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
      （海外への個人情報提供を含む）に同意する
    </span>
  );
}
