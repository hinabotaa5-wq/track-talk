import { redirect } from "next/navigation";
import { Suspense } from "react";
import { legacyCompetitionsTarget } from "@/lib/competition-prefecture";
import { CompetitionsHome } from "./competitions-home";

export default async function CompetitionsPage({
  searchParams,
}: {
  searchParams: Promise<{ month?: string; prefecture?: string }>;
}) {
  const sp = await searchParams;
  const target = legacyCompetitionsTarget(sp.prefecture, sp.month);
  if (target) redirect(target);

  return (
    <Suspense fallback={<p className="text-sm text-slate-200">読み込み中...</p>}>
      <CompetitionsHome />
    </Suspense>
  );
}
