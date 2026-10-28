import { PageLoading } from "@/components/page-loading";
import type { Metadata } from "next";
import { cookies } from "next/headers";
import { redirect } from "next/navigation";
import { Suspense } from "react";
import { competitionPrefectureCookie, regionalRedirectTarget } from "@/lib/competition-prefecture";
import { RegionalCompetitionsView } from "./regional-view";

export const metadata: Metadata = {
  title: "都道府県の大会",
};

export default async function RegionalCompetitionsPage({
  searchParams,
}: {
  searchParams: Promise<{ month?: string; prefecture?: string }>;
}) {
  const sp = await searchParams;
  const jar = await cookies();
  const target = regionalRedirectTarget(sp.prefecture, jar.get(competitionPrefectureCookie)?.value, sp.month);
  if (target) redirect(target);

  return (
    <Suspense fallback={<PageLoading label="読み込み中..." />}>
      <RegionalCompetitionsView />
    </Suspense>
  );
}
