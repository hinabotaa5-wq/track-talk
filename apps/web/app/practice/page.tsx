import type { Metadata } from "next";
import { Suspense } from "react";
import { PageLoading } from "@/components/page-loading";
import { PracticeListView } from "./practice-list-view";

export const metadata: Metadata = {
  title: "練習相手募集｜都道府県と種目で探す",
  description: "これから開かれる陸上競技の練習相手の募集を、都道府県と種目で探せます。",
  alternates: { canonical: "/practice" },
};

export default function PracticeListPage() {
  return (
    <Suspense fallback={<PageLoading label="読み込み中..." />}>
      <PracticeListView />
    </Suspense>
  );
}
