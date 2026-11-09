"use client";

import { CoachList } from "./coach-list";
import { SupportPlanList } from "./support-plan-list";
import { useRouter, useSearchParams } from "next/navigation";

// コーチタブの上部の切り替え。「動画添削」は指名用のコーチ一覧、「サポートプラン」は対応コーチの一覧（別のリスト）。
export function CoachTabs() {
  const params = useSearchParams();
  const router = useRouter();
  const tab = params.get("tab") === "support" ? "support" : "video";
  const tabs = [
    { id: "video", label: "動画添削" },
    { id: "support", label: "サポートプラン" },
  ] as const;
  return (
    <div className="space-y-4">
      <div role="tablist" aria-label="コーチの一覧を切り替える" className="grid grid-cols-2 gap-1 rounded-xl border border-white/20 bg-white/5 p-1">
        {tabs.map((t) => (
          <button key={t.id} type="button" role="tab" aria-selected={tab === t.id} onClick={() => router.replace(t.id === "support" ? "/coaches?tab=support" : "/coaches")} className={`rounded-lg px-3 py-2 text-sm font-semibold ${tab === t.id ? "bg-blue-600 text-white" : "text-slate-200"}`}>
            {t.label}
          </button>
        ))}
      </div>
      <div role="tabpanel">{tab === "support" ? <SupportPlanList /> : <CoachList />}</div>
    </div>
  );
}
