"use client";

import { useEffect, useState } from "react";
import { useAuth } from "@/components/auth-provider";
import { TrackLanesMark } from "@/components/app-icon-mark";

// 初期HTMLにも含め、JavaScriptの読み込み中からロゴを表示する。
// ルートレイアウトに置くので、アプリ内のタブ移動では表示し直さない。
export function StartupSplash() {
  const { loading } = useAuth();
  const [dismissed, setDismissed] = useState(false);

  useEffect(() => {
    // 認証通信が失敗しても画面全体をふさぎ続けない。
    const timeout = window.setTimeout(() => setDismissed(true), 10000);
    return () => window.clearTimeout(timeout);
  }, []);

  useEffect(() => {
    if (loading) return;
    const frame = window.requestAnimationFrame(() => setDismissed(true));
    return () => window.cancelAnimationFrame(frame);
  }, [loading]);

  if (dismissed) return null;

  return (
    <div
      id="startup-splash"
      role="status"
      aria-label="TrackTalkを起動しています"
      aria-busy="true"
      style={{
        position: "fixed", inset: 0, zIndex: 2147483647,
        display: "grid", placeItems: "center", background: "#ffffff",
        colorScheme: "light", pointerEvents: "none",
        animation: "startup-splash-release 0s 10s forwards",
      }}
    >
      <TrackLanesMark width="192" height="192" />
      <noscript><style>{"#startup-splash{display:none!important}"}</style></noscript>
    </div>
  );
}
