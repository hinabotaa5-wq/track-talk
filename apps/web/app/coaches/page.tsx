import type { Metadata } from "next";
import { CoachesView } from "./page-view";

export const metadata: Metadata = {
  title: "コーチ一覧",
  description: "トラックトーク（TrackTalk）のオンラインコーチングで見てくれるコーチの一覧です。記録・成績や指導経験を、依頼する前に確かめられます。", // i18n-data
};

// 文言は表示言語に合わせて画面側で描く（本体 #530）。
export default function Page() {
  return <CoachesView />;
}
