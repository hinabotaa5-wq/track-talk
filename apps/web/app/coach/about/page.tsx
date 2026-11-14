import type { Metadata } from "next";
import { CoachAboutView } from "./page-view";

export const metadata: Metadata = {
  title: "コーチ機能について",
  description: "陸上の経験を、次の一歩を支えるアドバイスに。TrackTalkのコーチの役割と、フォーム添削・継続プランの仕組みを紹介します。",
};

// 文言は表示言語に合わせて画面側で描く（本体 #530）。
export default function Page() {
  return <CoachAboutView />;
}
