import type { Metadata } from "next";
import { CoachingView } from "./page-view";

// 開始の告知は環境変数で出し分ける（lib/video-review-start.ts）。アクセスごとに描く設定は従来のまま残す。
export const dynamic = "force-dynamic";

export const metadata: Metadata = {
  title: "コーチング",
  description: "TrackTalk のオンラインコーチングで見てくれるコーチの一覧です。記録・成績や指導経験を、依頼する前に確かめられます。",
};

// 文言は表示言語に合わせて画面側で描く（本体 #530）。
export default function Page() {
  return <CoachingView />;
}
