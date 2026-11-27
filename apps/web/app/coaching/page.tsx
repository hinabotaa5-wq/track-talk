import type { Metadata } from "next";
import { CoachingGate } from "@/components/coaching-gate";
import { coachingCoachListEnabled, coachingEnabled, featuredCoachIDs } from "@/lib/features";
import { CoachesView } from "../coaches/page-view";
import { FeaturedCoachView } from "./featured-coach-view";

// 開始の告知は環境変数で出し分ける（lib/video-review-start.ts）。アクセスごとに描く設定は従来のまま残す。
export const dynamic = "force-dynamic";

// 特集モード（本体 #985）では、一覧ではなく特集コーチの紹介と依頼の入口の説明にする。
export const metadata: Metadata = {
  title: "コーチング",
  description: featuredCoachIDs.length
    ? "トラックトーク（TrackTalk）のオンラインコーチング。走りの動画を1本送ると、コーチが良い点・直す点・おすすめのドリルを24時間以内に返します。" // i18n-data
    : "トラックトーク（TrackTalk）のオンラインコーチングで見てくれるコーチの一覧です。記録・成績や指導経験を、依頼する前に確かめられます。", // i18n-data
};

// 文言は表示言語に合わせて画面側で描く（本体 #530）。
// 下部ナビの「コーチ」はこの一覧を開く。/coaches と同じ一覧画面を共有する（/coaches は next.config で /coaching へ転送）。
// 特集モードの間は一覧の代わりに特集ページを出す（本体 #985）。特集コーチが複数ならプロフィールを横スライドで切り替える（本体 #1063）。
// 一覧フラグのゲートはこのルートだけに掛け、配下（/coaching/new など）の現行ゲートは変えない。
export default function Page() {
  return (
    <CoachingGate announceVideoReviewStart open={coachingEnabled && coachingCoachListEnabled}>
      {featuredCoachIDs.length ? <FeaturedCoachView coachIDs={featuredCoachIDs} /> : <CoachesView />}
    </CoachingGate>
  );
}
