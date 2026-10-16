import type { Metadata } from "next";

// 保護者の同意ページはリンクを知っている人だけが開く。検索に載せない。
export const metadata: Metadata = {
  title: "保護者の同意 | TrackTalk",
  robots: { index: false, follow: false },
};

export default function GuardianLayout({ children }: { children: React.ReactNode }) {
  return children;
}
