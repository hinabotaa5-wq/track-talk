"use client";

import { usePathname } from "next/navigation";
import { useEffect, useState } from "react";
import { useAuth } from "@/components/auth-provider";
import { profileGateRedirecting } from "@/lib/profile-gate";
import { TrackLanesMark } from "@/components/app-icon-mark";

// 明るい表示の背景と同じ白。manifest の background_color と iOS の起動画像もこの色にそろえ、
// OSの起動画面からアプリ内の起動画面へ切り替わるときに色が変わらないようにする。
// アプリ内の起動画面は、端末がダークモードなら暗い背景(globals.css の --background)にする。
export const startupSplashBackground = "#ffffff";
// 閉じるときのフェードの長さ。globals.css の transition と合わせる。
const fadeMs = 400;

// 初期HTMLにも含め、JavaScriptの読み込み中からロゴを表示する。
// ルートレイアウトに置くので、アプリ内のタブ移動では表示し直さない。
export function StartupSplash() {
  const { loading, profile } = useAuth();
  const pathname = usePathname();
  // 登録したばかりでニックネームや同意がまだの人は、ProfileGate が決める画面へ移し終えるまで閉じない
  // (ホームが一瞬見えてから移るのを防ぐ)。移れなかったときも、下の10秒で閉じる。
  const holding = !loading && profileGateRedirecting(pathname, profile);
  const [phase, setPhase] = useState<"shown" | "leaving" | "gone">("shown");

  useEffect(() => {
    // 認証通信が失敗しても画面全体をふさぎ続けない。
    const timeout = window.setTimeout(() => setPhase((p) => (p === "shown" ? "leaving" : p)), 10000);
    return () => window.clearTimeout(timeout);
  }, []);

  useEffect(() => {
    if (loading || holding) return;
    const frame = window.requestAnimationFrame(() => setPhase((p) => (p === "shown" ? "leaving" : p)));
    return () => window.cancelAnimationFrame(frame);
  }, [loading, holding]);

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
        // 容器は全面を覆うだけにし、中央寄せはしない。容器の高さはモバイルの
        // ツールバーの出入りで変わり、grid 中央のロゴが上下に動いてしまうため。
        display: "block", background: `var(--background, ${startupSplashBackground})`, pointerEvents: "none",
        animation: "startup-splash-release 400ms 10s forwards",
      }}
    >
      {/* ロゴの位置は容器の高さではなく小さい側の画面高さ(50svh)を基準にする。
          ツールバーが隠れて容器が伸びても、最初の表示位置から動かない。
          svh 未対応の古い端末では globals.css の 50vh が替わりに効く。 */}
      <div
        id="startup-splash-logo"
        style={{
          position: "absolute", top: "50svh", left: "50%",
          transform: "translate(-50%, -50%)",
        }}
      >
        <TrackLanesMark width="192" height="192" />
      </div>
      <noscript><style>{"#startup-splash{display:none!important}"}</style></noscript>
    </div>
  );
}
