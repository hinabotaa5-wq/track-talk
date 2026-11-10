import type { Metadata } from "next";
import { ContactView } from "./page-view";

export const metadata: Metadata = {
  title: "運営者・お問い合わせ",
  description: "TrackTalkの運営者と、ログインせずに利用できるお問い合わせ先。",
};

// 文言は表示言語に合わせて画面側で描く（本体 #530）。
export default function Page() {
  return <ContactView />;
}
