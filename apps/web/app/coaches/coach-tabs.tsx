"use client";
import { L } from "@/lib/i18n";

import { useEffect, useState } from "react";
import { motion, MotionConfig } from "motion/react";
import { ViewSwitch } from "@/components/view-switch";
import { CoachList } from "./coach-list";
import { CoachSelfButton } from "./coach-self-button";
import { usePathname } from "next/navigation";

// コーチ一覧の上部の切り替え。スタンダードの指名用のコーチ一覧だけを出す。
// 継続プランは商品として休眠させたので（本体 #1007）、継続プランの選択肢と対応コーチの一覧（SupportPlanList）は描画しない。
// ?tab=support の直接リンク・共有 URL で開かれても、つねにスタンダード側を出す。
// SupportPlanList の部品とフラグ（NEXT_PUBLIC_SUPPORT_PLAN_LIST_ENABLED）は休眠として残す。
type Tab = "video";

export function CoachTabs({ onLoadingChange }: { onLoadingChange?: (loading: boolean) => void }) {
  const pathname = usePathname();
  const base = pathname === "/coaching" ? "/coaching" : "/coaches";
  const tab: Tab = "video";
  // 初回読み込み中はページ中身をストップウォッチだけにする（本体 #804）。
  // 一覧は初回の取得1回だけを行い、終わったら（成功・失敗のどちらも）知らせる。
  const [settled, setSettled] = useState(false);
  const loading = !settled;
  useEffect(() => {
    onLoadingChange?.(loading);
  }, [loading, onLoadingChange]);
  const views = [{ id: "video", label: L("スタンダード") }] as const;
  return (
    <div className="space-y-4">
      <MotionConfig reducedMotion="user">
        {loading ? null : (
          <div className="flex items-center gap-2">
            <div className="min-w-0 flex-1">
              <ViewSwitch
                label={L("コーチの一覧を切り替える")}
                layoutId="coach-list-view-pill"
                initialPillStatic
                views={views}
                current={tab}
                onSelect={() => {}}
                href={() => base}
              />
            </div>
            <CoachSelfButton />
          </div>
        )}
        <div role="tabpanel">
          <motion.div
            key={tab}
            initial={{ opacity: 0, y: 8 }}
            animate={{ opacity: 1, y: 0 }}
            transition={{ duration: 0.22, ease: "easeOut" }}
          >
            <CoachList onSettled={() => setSettled(true)} />
          </motion.div>
        </div>
      </MotionConfig>
    </div>
  );
}
