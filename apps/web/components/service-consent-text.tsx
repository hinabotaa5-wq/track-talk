import { L } from "@/lib/i18n";
import Link from "next/link";

// 登録と、ログインから初めて利用する場合の同意を同じ文言にそろえる。
export function ServiceConsentText() {
  return (
    <span>
      <Link href="/terms" className="text-blue-400 underline">{L("利用規約")}</Link>{L("・")}<Link href="/guidelines" className="text-blue-400 underline">{L("コミュニティガイドライン")}</Link>{L("・")}<Link href="/privacy" className="text-blue-400 underline">{L("プライバシーポリシー")}</Link>{L("（海外への個人情報提供を含む）に同意する")}</span>
  );
}
