import { cookies } from "next/headers";
import { redirect } from "next/navigation";
import { Suspense } from "react";
import { competitionPrefectureCookie, competitionsRedirectTarget } from "@/lib/competition-prefecture";
import { CompetitionsView } from "./competitions-view";

export default async function CompetitionsPage({
  searchParams,
}: {
  searchParams: Promise<{ month?: string; prefecture?: string }>;
}) {
  const sp = await searchParams;
  const jar = await cookies();
  const target = competitionsRedirectTarget(
    sp.prefecture,
    jar.get(competitionPrefectureCookie)?.value,
    sp.month,
  );
  if (target) redirect(target);

  return (
    <Suspense fallback={<p className="text-sm text-zinc-600">読み込み中...</p>}>
      <CompetitionsView />
    </Suspense>
  );
}
