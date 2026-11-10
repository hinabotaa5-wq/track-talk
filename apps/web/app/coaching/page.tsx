import type { Metadata } from "next";
import { CoachingView } from "./page-view";

// 開始予定日の案内は、ビルド時ではなくアクセス時の日時で出し分ける。
export const dynamic = "force-dynamic";

export const metadata: Metadata = {
  title: "コーチング",
  description: "コーチングは準備中です。現在は陸上競技のコーチを募集しています。",
};

// 文言は表示言語に合わせて画面側で描く（本体 #530）。
export default function Page() {
  return <CoachingView />;
}
