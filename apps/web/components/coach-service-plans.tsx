"use client";
import { L } from "@/lib/i18n";

import type { CoachingPlan } from "@/lib/types";
import type { PublicSupportConsult } from "@/lib/support-consult";
import { PublicPlanList } from "./coaching-plans";
import { SupportConsultEntry } from "./support-consult-entry";

// コーチ詳細の「サポートプラン」。種別ごとの切り分けはしない。単一のセクションに、受け入れステータス・
// 価格目安・標準形・無料相談ボタンを出す。対面プランは出さない（コード・データは残し、表示だけしない）。
export function CoachServicePlans({ plans, coachID, consult }: { plans: CoachingPlan[]; coachID?: string; consult?: PublicSupportConsult | null }) {
  const consultEntry = coachID && consult ? consult : null;
  const legacy = plans.filter((plan) => plan.published && !plan.service_type);

  return (
    <div className="space-y-4">
      {consultEntry ? (
        <section aria-label={L("サポートプラン")} className="space-y-3">
          <h3 className="font-bold text-white">{L("サポートプラン")}</h3>
          <SupportConsultEntry coachID={coachID!} consult={consultEntry} />
        </section>
      ) : (
        <p className="text-sm text-slate-200">{L("このコーチは受付していません")}</p>
      )}
      {legacy.length ? <section className="space-y-2" aria-label={L("従来のサポートプラン")}>
        <h3 className="text-sm font-semibold">{L("従来のサポートプラン")}</h3>
        <PublicPlanList plans={legacy} />
      </section> : null}
    </div>
  );
}
