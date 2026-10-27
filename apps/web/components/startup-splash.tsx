"use client";

import { useEffect, useState } from "react";
import { useAuth } from "@/components/auth-provider";
import { TrackLanesMark } from "@/components/app-icon-mark";

// ヘッダーと同じ濃紺。manifest の background_color と iOS の起動画像もこの色にそろえ、
// OSの起動画面からアプリ内の起動画面へ切り替わるときに色が変わらないようにする。
export const startupSplashBackground = "#16243a";
// 閉じるときのフェードの長さ。globals.css の transition と合わせる。
const fadeMs = 400;

// 初期HTMLにも含め、JavaScriptの読み込み中からロゴを表示する。
// ルートレイアウトに置くので、アプリ内のタブ移動では表示し直さない。
export function StartupSplash() {
  const { loading } = useAuth();
  const [phase, setPhase] = useState<"shown" | "leaving" | "gone">("shown");

  useEffect(() => {
    // 認証通信が失敗しても画面全体をふさぎ続けない。
    const timeout = window.setTimeout(() => setPhase((p) => (p === "shown" ? "leaving" : p)), 10000);
    return () => window.clearTimeout(timeout);
  }, []);

  useEffect(() => {
    if (loading) return;
    const frame = window.requestAnimationFrame(() => setPhase((p) => (p === "shown" ? "leaving" : p)));
    return () => window.cancelAnimationFrame(frame);
  }, [loading]);

  useEffect(() => {
    if (phase !== "leaving") return;
    // フェードを見せ終えてから外す。transitionend は省略される場合があるのでタイマーで待つ。
    const timeout = window.setTimeout(() => setPhase("gone"), fadeMs);
    return () => window.clearTimeout(timeout);
  }, [phase]);

  if (phase === "gone") return null;

  return (
    <div
      id="startup-splash"
      role="status"
      aria-label="TrackTalkを起動しています"
      aria-busy={phase === "shown"}
      data-state={phase}
      style={{
        position: "fixed", inset: 0, zIndex: 2147483647,
        display: "grid", placeItems: "center", background: startupSplashBackground,
        colorScheme: "dark", pointerEvents: "none",
        animation: "startup-splash-release 400ms 10s forwards",
      }}
    >
      <TrackLanesMark width="192" height="192" />
      <noscript><style>{"#startup-splash{display:none!important}"}</style></noscript>
    </div>
  );
}
