"use client";
import { L } from "@/lib/i18n";

import { CoachList } from "./coach-list";
import { SupportPlanList } from "./support-plan-list";
import { CoachButton } from "./coach-button";
import { ViewSwitch } from "@/components/view-switch";
import { usePathname, useRouter, useSearchParams } from "next/navigation";

// コーチタブの上部の切り替え。「動画添削」は指名用のコーチ一覧、「サポートプラン」は対応コーチの一覧（別のリスト）。
// 共通部品の ViewSwitch を使い、開いている一覧ルート（/coaching・/coaches）を保ったまま tab クエリだけを変える。
// ?tab=support の直接表示と共有 URL はそのまま保つ。
export function CoachTabs() {
  const params = useSearchParams();
  const pathname = usePathname();
  const router = useRouter();
  const tab = params.get("tab") === "support" ? "support" : "video";
  const href = (id: "video" | "support") => (id === "support" ? `${pathname}?tab=support` : pathname);
  return (
    <div className="space-y-4">
      <div className="flex items-center gap-2">
        <div className="min-w-0 flex-1">
          <ViewSwitch
            label={L("コーチの一覧を切り替える")}
            layoutId="coach-list-view-pill"
            views={[
              { id: "video", label: L("動画添削") },
              { id: "support", label: L("サポートプラン") },
            ] as const}
            current={tab}
            onSelect={(next) => router.replace(href(next))}
            href={href}
          />
        </div>
        <CoachButton />
      </div>
      <div role="tabpanel">{tab === "support" ? <SupportPlanList /> : <CoachList />}</div>
    </div>
  );
}
