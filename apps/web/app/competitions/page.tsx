import { PageLoading } from "@/components/page-loading";
import type { Metadata } from "next";
import { redirect } from "next/navigation";
import { Suspense } from "react";
import { apiFetch } from "@/lib/api";
import { legacyCompetitionsTarget } from "@/lib/competition-prefecture";
import type { DomesticMajor } from "@/lib/types";
import { CompetitionsHome } from "./competitions-home";

export const metadata: Metadata = {
  title: "陸上競技の大会日程",
  description: "インターハイ・インカレ・日本選手権・駅伝・マラソンなど、今シーズンの国内主要大会と国際大会の日程を月ごとにまとめています。",
  alternates: { canonical: "/competitions" },
};

export default async function CompetitionsPage({
  searchParams,
}: {
  searchParams: Promise<{ month?: string; prefecture?: string }>;
}) {
  const sp = await searchParams;
  const target = legacyCompetitionsTarget(sp.prefecture, sp.month);
  if (target) redirect(target);

  // 国内主要大会の一覧をサーバーで描いておき、検索エンジンが中身を読めるようにする。取れなければ画面側で取り直す。
  // 一覧はめったに変わらないので24時間取り置き、タブを開くたびに API の起動を待たない。画面側では最新を読み直す。
  const majors = await apiFetch<{ majors: DomesticMajor[] }>("/v1/domestic-majors", { signal: AbortSignal.timeout(3000), next: { revalidate: 86400 } })
    .then((data) => data.majors ?? [])
    .catch(() => null);

  // 読み込み中に中身が1画面に収まると、ページがスクロールできなくなり、
  // iPhone の Safari が下のツールバーを出し直して、下部バーが上下に動く。
  // 他のタブと同じく、読み込み中も1画面より高くしておく。
  return (
    <div className="min-h-svh md:min-h-0">
      <Suspense fallback={<PageLoading label="読み込み中..." />}>
        <CompetitionsHome initialMajors={majors} />
      </Suspense>
    </div>
  );
}
