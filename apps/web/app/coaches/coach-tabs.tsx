"use client";
import { L } from "@/lib/i18n";

import { ViewSwitch } from "@/components/view-switch";
import { CoachList } from "./coach-list";
import { SupportPlanList } from "./support-plan-list";
import { CoachSelfButton } from "./coach-self-button";
import { usePathname, useRouter, useSearchParams } from "next/navigation";

// コーチ一覧の上部の切り替え。「動画添削」は指名用のコーチ一覧、「サポートプラン」は対応コーチの一覧（別のリスト）。
// /coaching と /coaches のどちらの一覧でも同じ部品を使い、切り替えは今いるルートのまま tab だけを変える。
export function CoachTabs() {
  const params = useSearchParams();
  const router = useRouter();
  const pathname = usePathname();
  const base = pathname === "/coaching" ? "/coaching" : "/coaches";
  const tab = params.get("tab") === "support" ? "support" : "video";
  const views = [
    { id: "video", label: L("動画添削") },
    { id: "support", label: L("サポートプラン") },
  ] as const;
  type Tab = (typeof views)[number]["id"];
  function select(next: Tab) {
    router.replace(next === "support" ? `${base}?tab=support` : base);
  }
  return (
    <div className="space-y-4">
      <div className="flex items-center gap-2">
        <div className="min-w-0 flex-1">
          <ViewSwitch
            label={L("コーチの一覧を切り替える")}
            layoutId="coach-list-view-pill"
            views={views}
            current={tab}
            onSelect={select}
            href={(id) => (id === "support" ? `${base}?tab=support` : base)}
          />
        </div>
        <CoachSelfButton />
      </div>
      <div role="tabpanel">{tab === "support" ? <SupportPlanList /> : <CoachList />}</div>
    </div>
  );
}
