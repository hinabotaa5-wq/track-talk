"use client";
import { L } from "@/lib/i18n";

import { useState } from "react";
import { motion, MotionConfig } from "motion/react";
import { ViewSwitch } from "@/components/view-switch";
import { CoachList } from "./coach-list";
import { SupportPlanList } from "./support-plan-list";
import { CoachSelfButton } from "./coach-self-button";
import { usePathname, useSearchParams } from "next/navigation";

// コーチ一覧の上部の切り替え。「動画添削」は指名用のコーチ一覧、「サポートプラン」は対応コーチの一覧（別のリスト）。
// /coaching と /coaches のどちらの一覧でも同じ部品を使い、切り替えは今いるルートのまま tab だけを変える。
// 大会・ケガのタブと同じく、その場の状態で即座に切り替え、URL は書き換えるだけにする。
// router.replace だとサーバーまで取りに行って「読み込み中」が挟まるので使わない。
// 一度開いた一覧は消さずに隠しておき、行き来しても読み込み直さない。
type Tab = "video" | "support";

export function CoachTabs() {
  const params = useSearchParams();
  const pathname = usePathname();
  const base = pathname === "/coaching" ? "/coaching" : "/coaches";
  // 直接リンク・共有 URL でサポートプラン側から開けるよう、初回だけ URL を見る。以後はその場の状態が正本。
  const initial: Tab = params.get("tab") === "support" ? "support" : "video";
  const [tab, setTab] = useState<Tab>(initial);
  // 一度開いたパネルは消さずに隠しておき、行き来しても読み込み直さない（大会・ケガと同じ）。
  const [opened, setOpened] = useState<Set<Tab>>(() => new Set([initial]));
  const views = [
    { id: "video", label: L("動画添削") },
    { id: "support", label: L("サポートプラン") },
  ] as const;
  function select(next: Tab) {
    if (next === tab) return;
    setOpened((prev) => (prev.has(next) ? prev : new Set(prev).add(next)));
    setTab(next);
    window.history.replaceState(null, "", next === "support" ? `${base}?tab=support` : base);
  }
  return (
    <div className="space-y-4">
      <MotionConfig reducedMotion="user">
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
        <div role="tabpanel">
          {views.map((item) =>
            opened.has(item.id) || item.id === tab ? (
              <motion.div
                key={item.id}
                hidden={item.id !== tab}
                initial={{ opacity: 0, y: 8 }}
                animate={item.id === tab ? { opacity: 1, y: 0 } : { opacity: 0, y: 8 }}
                transition={{ duration: 0.22, ease: "easeOut" }}
              >
                {item.id === "support" ? <SupportPlanList /> : <CoachList />}
              </motion.div>
            ) : null,
          )}
        </div>
      </MotionConfig>
    </div>
  );
}
