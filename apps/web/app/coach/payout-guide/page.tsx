import type { Metadata } from "next";
import { CoachPayoutGuideView } from "./page-view";

export const metadata: Metadata = {
  title: "受け取り口座の登録手順",
  description: "コーチの受け取り口座（Stripe）の登録手順。事業形態・業種・ウェブサイト・商品の説明の入力例と、完了の確認方法を案内します。",
};

// 文言は表示言語に合わせて画面側で描く（本体 #530）。
export default function Page() {
  return <CoachPayoutGuideView />;
}
