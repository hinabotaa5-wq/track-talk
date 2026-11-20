import type { Metadata } from "next";
import { ReferralGuideView } from "./page-view";

export const metadata: Metadata = {
  title: "紹介で1回無料",
  description: "購入すると、紹介IDが発行されます。そのIDで友達2人が購入すると、紹介してくれた方に次回の動画添削1回分が無料になるクーポンをプレゼントします。クーポンの有効期限は、プレゼントされてから6か月です。",
};

// 文言は表示言語に合わせて画面側で描く（本体 #530）。
export default function Page() {
  return <ReferralGuideView />;
}
