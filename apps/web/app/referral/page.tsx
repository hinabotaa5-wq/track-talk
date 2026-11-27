import type { Metadata } from "next";
import { ReferralGuideView } from "./page-view";

export const metadata: Metadata = {
  title: "2人に紹介すると500円分無料",
  description: "購入すると、紹介IDが発行されます。そのIDで友達2人が購入すると、紹介してくれた方に¥500クーポンがプレゼントされます。クーポンは申込みで使えます。有効期限は、プレゼントされてから6か月です。",
};

// 文言は表示言語に合わせて画面側で描く（本体 #530）。
export default function Page() {
  return <ReferralGuideView />;
}
