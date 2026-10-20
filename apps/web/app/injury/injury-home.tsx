"use client";

import { useState } from "react";
import { motion, MotionConfig } from "motion/react";
import { ViewSwitch } from "@/components/view-switch";
import { InjuryView } from "./injury-view";
import { DataView } from "./data/data-view";

type View = "share" | "data";

const views: { id: View; label: string }[] = [
  { id: "share", label: "共有" },
  { id: "data", label: "データ" },
];

// ケガタブ。「共有」と「データ」を、読み込み直さずにその場で切り替える(大会タブと同じ)。
export function InjuryHome({ initialView }: { initialView: View }) {
  const [view, setView] = useState<View>(initialView);
  // 一度開いた方は消さずに隠しておき、行き来しても読み込み直さない。
  const [opened, setOpened] = useState<Set<View>>(() => new Set([initialView]));

  // router.replace だとサーバーまで取りに行って「読み込み中」が挟まるので、URL だけ書き換える。
  function select(next: View) {
    if (next === view) return;
    setOpened((prev) => (prev.has(next) ? prev : new Set(prev).add(next)));
    setView(next);
    window.history.replaceState(null, "", next === "share" ? "/injury" : "/injury/data");
  }

  return (
    <div className="space-y-4">
      <h1 className="sr-only">ケガ</h1>
      <MotionConfig reducedMotion="user">
        <ViewSwitch label="ケガタブの切り替え" layoutId="injury-view-pill" views={views} current={view} onSelect={select} />
        {views.map((item) =>
          opened.has(item.id) || item.id === view ? (
            <motion.div
              key={item.id}
              hidden={item.id !== view}
              initial={{ opacity: 0, y: 8 }}
              animate={item.id === view ? { opacity: 1, y: 0 } : { opacity: 0, y: 8 }}
              transition={{ duration: 0.22, ease: "easeOut" }}
            >
              {item.id === "share" ? <InjuryView /> : <DataView />}
            </motion.div>
          ) : null,
        )}
      </MotionConfig>
    </div>
  );
}
