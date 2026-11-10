import type { Metadata } from "next";
import { GoodbyeView } from "./page-view";

export const metadata: Metadata = {
  title: "退会しました",
  robots: { index: false },
};

// 文言は表示言語に合わせて画面側で描く（本体 #530）。
export default function Page() {
  return <GoodbyeView />;
}
