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

  // 読み込み中に中身が1画面に収まると、ページがスクロールできなくなり、
  // iPhone の Safari が下のツールバーを出し直して、下部バーが上下に動く。
  // 他のタブと同じく、読み込み中も1画面より高くしておく。
  return (
    <div className="min-h-svh md:min-h-0">
      <Suspense fallback={<p className="text-sm text-slate-200">読み込み中...</p>}>
        <CompetitionsHome />
      </Suspense>
    </div>
  );
}
