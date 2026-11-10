import type { Metadata } from "next";
import { GuidelinesView } from "./page-view";

export const metadata: Metadata = {
  title: "コミュニティガイドライン",
  description: "TrackTalk のコミュニティガイドライン",
};

// 文言は表示言語に合わせて画面側で描く（本体 #530）。
export default function Page() {
  return <GuidelinesView />;
}
