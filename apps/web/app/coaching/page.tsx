import type { Metadata } from "next";
import { CoachingGate } from "@/components/coaching-gate";
import { coachingCoachListEnabled, coachingEnabled } from "@/lib/features";
import { CoachesView } from "../coaches/page-view";

// 開始の告知は環境変数で出し分ける（lib/video-review-start.ts）。アクセスごとに描く設定は従来のまま残す。
export const dynamic = "force-dynamic";

export const metadata: Metadata = {
  title: "コーチング",
  description: "トラックトーク（TrackTalk）のオンラインコーチングで見てくれるコーチの一覧です。記録・成績や指導経験を、依頼する前に確かめられます。", // i18n-data
};

// 文言は表示言語に合わせて画面側で描く（本体 #530）。
// 下部ナビの「コーチ」はこの一覧を開く。/coaches と同じ一覧画面を共有する。
// 一覧フラグのゲートはこのルートだけに掛け、配下（/coaching/new など）の現行ゲートは変えない。
export default function Page() {
  return (
    <CoachingGate announceVideoReviewStart open={coachingEnabled && coachingCoachListEnabled}>
      <CoachesView />
    </CoachingGate>
  );
}
