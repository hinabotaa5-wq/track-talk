"use client";
import { L } from "@/lib/i18n";

import { ChevronDown, MapPin, Trophy } from "lucide-react";
import { useId, useState } from "react";
import { coachingServiceLabels } from "@/lib/coaching-plan";
import type { CoachingPlan, CoachingServiceType } from "@/lib/types";
import type { PublicSupportConsult } from "@/lib/support-consult";
import { PublicPlanList } from "./coaching-plans";
import { SupportConsultEntry } from "./support-consult-entry";

const choices = [
  { service: "competition_support", icon: Trophy, get description() { return L("目標の大会まで、練習や調整を継続して相談する"); } },
  { service: "in_person", icon: MapPin, get description() { return L("コーチと現地で会い、動きを見てもらいながら指導を受ける"); } },
] as const;

// consult があるコーチの大会までプランは、価格の決まった完成品ではなく、無料相談の入口を出す。
export function CoachServicePlans({ services, plans, coachID, consult }: { services: CoachingServiceType[]; plans: CoachingPlan[]; coachID?: string; consult?: PublicSupportConsult | null }) {
  const [selected, setSelected] = useState<CoachingServiceType | null>(null);
  const panelID = useId();
  const publicPlans = plans.filter((plan) => plan.published && plan.service_type !== "video_review");
  const consultEntry = coachID && consult ? consult : null;
  const available = (service: CoachingServiceType) => (service === "competition_support" && consultEntry !== null) || (services.includes(service) && publicPlans.some((plan) => plan.service_type === service));
  const active = selected && available(selected) ? selected : null;
  const legacy = publicPlans.filter((plan) => !plan.service_type);

  return (
    <div className="space-y-4">
      <p className="text-sm text-slate-200">{L("受けたい指導を選ぶと、内容と料金を確認できます。")}</p>
      <div className="space-y-2" aria-label={L("指導の種類")}>
        {choices.map(({ service, icon: Icon, description }) => {
          const enabled = available(service);
          const expanded = active === service;
          return (
            <button key={service} type="button" disabled={!enabled}
              aria-expanded={expanded} aria-controls={panelID}
              onClick={() => setSelected(expanded ? null : service)}
              className={`flex min-h-20 w-full items-center gap-3 rounded-xl border p-4 text-left transition disabled:cursor-not-allowed disabled:border-white/10 disabled:bg-white/5 disabled:text-slate-500 ${expanded ? "border-blue-400 bg-blue-500/20 text-pure-white" : "border-white/20 bg-white/10 text-slate-100 enabled:hover:border-blue-400/60 enabled:hover:bg-white/15"}`}>
              <Icon className={`size-6 shrink-0 ${enabled ? "text-blue-300" : "text-slate-500"}`} aria-hidden />
              <span className="min-w-0 flex-1 space-y-1">
                <span className="block font-semibold">{coachingServiceLabels[service]}</span>
                <span className={`block text-xs leading-relaxed ${enabled ? "text-slate-200" : "text-slate-500"}`}>{description}</span>
                {!enabled ? <span className="block text-xs">{services.includes(service) ? L("公開中のプランがありません") : L("このコーチは受付していません")}</span> : null}
              </span>
              <ChevronDown className={`size-4 shrink-0 transition-transform ${expanded ? "rotate-180" : ""}`} aria-hidden />
            </button>
          );
        })}
      </div>
      <div id={panelID} hidden={!active}>
        {active ? <section aria-label={L("{v0}のプラン", { v0: coachingServiceLabels[active] })} className="space-y-3">
          <h3 className="font-bold text-white">{coachingServiceLabels[active]}</h3>
          {active === "competition_support" && consultEntry ? <SupportConsultEntry coachID={coachID!} consult={consultEntry} /> : <PublicPlanList plans={publicPlans.filter((plan) => plan.service_type === active)} />}
        </section> : null}
      </div>
      {legacy.length ? <section className="space-y-2" aria-label={L("従来のサポートプラン")}>
        <h3 className="text-sm font-semibold">{L("従来のサポートプラン")}</h3>
        <PublicPlanList plans={legacy} />
      </section> : null}
    </div>
  );
}
