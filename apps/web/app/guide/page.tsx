import type { Metadata } from "next";
import { GuideView } from "./page-view";

export const metadata: Metadata = { title: "利用ガイド", description: "TrackTalkの投稿・大会・メモ・ケガの使い方とコーチ募集の案内。" };

// 文言は表示言語に合わせて画面側で描く（本体 #530）。
export default function Page() {
  return <GuideView />;
}
