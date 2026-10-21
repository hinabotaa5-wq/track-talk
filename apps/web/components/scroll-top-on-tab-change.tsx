"use client";

import { useEffect, useRef } from "react";
import { usePathname } from "next/navigation";
import { activeMainTab } from "./bottom-nav";

// 下のメニューのタブを別のタブに移ったら、前のタブのスクロール位置を引き継がず、いちばん上から表示する。
// 詳細画面からの戻るでは動かさない(戻る前の位置を残す)ため、前後どちらもタブの画面のときだけ動かす。
export function ScrollTopOnTabChange() {
  const tab = activeMainTab(usePathname());
  const prev = useRef(tab);

  useEffect(() => {
    if (prev.current && tab && prev.current !== tab) window.scrollTo(0, 0);
    prev.current = tab;
  }, [tab]);

  return null;
}
