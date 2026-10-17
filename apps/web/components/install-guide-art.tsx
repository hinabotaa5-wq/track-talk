import type { ReactNode } from "react";
import { TrackLanesMark } from "./app-icon-mark";

// 手順ごとの説明イラスト。iOS / Android の実画面は OS のバージョンで変わるため、
// 要点だけを描いた簡易版にしている。実画面の画像に替える場合はここを差し替える。

const accent = "#38bdf8";

function Frame({ label, children }: { label: string; children: ReactNode }) {
  return (
    <svg
      viewBox="0 0 240 132"
      role="img"
      aria-label={label}
      className="h-auto w-full rounded-xl border border-white/15 bg-slate-900/60"
    >
      {children}
    </svg>
  );
}

function Pulse({ cx, cy, r = 16 }: { cx: number; cy: number; r?: number }) {
  return (
    <circle cx={cx} cy={cy} r={r} fill="none" stroke={accent} strokeWidth="2.5">
      <animate attributeName="r" values={`${r};${r + 6};${r}`} dur="1.6s" repeatCount="indefinite" />
      <animate attributeName="opacity" values="1;0.35;1" dur="1.6s" repeatCount="indefinite" />
    </circle>
  );
}

function ShareGlyph({ x, y, color = "#e2e8f0" }: { x: number; y: number; color?: string }) {
  return (
    <g transform={`translate(${x} ${y})`} fill="none" stroke={color} strokeWidth="2" strokeLinecap="round" strokeLinejoin="round">
      <path d="M-6 -1v9a2 2 0 0 0 2 2h8a2 2 0 0 0 2-2v-9" />
      <path d="M0 5v-14M-4 -5l4-4 4 4" />
    </g>
  );
}

export function OpenInSafariArt() {
  return (
    <Frame label="Safari のアイコンをタップして開く">
      <rect x="20" y="22" width="200" height="30" rx="15" fill="#334155" />
      <text x="120" y="42" textAnchor="middle" fontSize="12" fill="#cbd5e1">tracktalk の URL</text>
      <circle cx="120" cy="92" r="22" fill="#f8fafc" />
      <circle cx="120" cy="92" r="17" fill="none" stroke="#2563eb" strokeWidth="3" />
      <path d="M120 80l5 12-5 12-5-12z" fill="#ef4444" />
      <Pulse cx={120} cy={92} r={26} />
    </Frame>
  );
}

export function IosShareButtonArt() {
  return (
    <Frame label="Safari の画面下の共有ボタンをタップ">
      <rect x="40" y="8" width="160" height="62" rx="8" fill="#1e293b" />
      <text x="120" y="42" textAnchor="middle" fontSize="12" fill="#94a3b8">TrackTalk</text>
      <rect x="0" y="86" width="240" height="46" fill="#334155" />
      <path d="M28 109l8-7v14zM58 109l-8-7v14z" fill="none" stroke="#94a3b8" strokeWidth="2" strokeLinejoin="round" />
      <ShareGlyph x={120} y={105} color="#fff" />
      <rect x="172" y="101" width="14" height="14" rx="2" fill="none" stroke="#94a3b8" strokeWidth="2" />
      <rect x="206" y="101" width="14" height="14" rx="2" fill="none" stroke="#94a3b8" strokeWidth="2" />
      <Pulse cx={120} cy={107} r={17} />
      <path d="M120 78v8" stroke={accent} strokeWidth="2.5" strokeLinecap="round" />
    </Frame>
  );
}

export function IosAddToHomeArt() {
  return (
    <Frame label="メニューから「ホーム画面に追加」を選ぶ">
      <rect x="30" y="8" width="180" height="116" rx="12" fill="#1e293b" />
      <g fontSize="11" fill="#94a3b8">
        <text x="46" y="32">コピー</text>
        <text x="46" y="56">ブックマークを追加</text>
        <text x="46" y="106">プリント</text>
      </g>
      <rect x="36" y="64" width="168" height="26" rx="7" fill="#2563eb" opacity="0.9" />
      <text x="46" y="81" fontSize="11" fontWeight="700" fill="#fff">ホーム画面に追加</text>
      <rect x="176" y="70" width="14" height="14" rx="3" fill="none" stroke="#fff" strokeWidth="1.8" />
      <path d="M183 73v8M179 77h8" stroke="#fff" strokeWidth="1.8" strokeLinecap="round" />
      <path d="M212 77h-14" stroke={accent} strokeWidth="2.5" strokeLinecap="round" />
      <path d="M200 72l-6 5 6 5" fill="none" stroke={accent} strokeWidth="2.5" strokeLinecap="round" strokeLinejoin="round" />
    </Frame>
  );
}

export function IosConfirmArt() {
  return (
    <Frame label="右上の「追加」をタップ">
      <rect x="20" y="8" width="200" height="116" rx="12" fill="#1e293b" />
      <text x="30" y="30" fontSize="8" fill="#94a3b8">キャンセル</text>
      <text x="112" y="30" textAnchor="middle" fontSize="8.5" fontWeight="700" fill="#e2e8f0">ホーム画面に追加</text>
      <rect x="182" y="17" width="28" height="18" rx="9" fill="#2563eb" />
      <text x="196" y="30" textAnchor="middle" fontSize="10" fontWeight="700" fill="#fff">追加</text>
      <Pulse cx={196} cy={26} r={17} />
      <rect x="34" y="50" width="44" height="44" rx="10" fill="#16243a" stroke="#ffffff" strokeOpacity="0.15" />
      <TrackLanesMark x="43.7" y="60.9" width="24.6" height="22.3" />
      <text x="90" y="68" fontSize="12" fontWeight="700" fill="#e2e8f0">TrackTalk</text>
      <text x="90" y="84" fontSize="9" fill="#94a3b8">tracktalk のアドレス</text>
    </Frame>
  );
}

export function AndroidMenuArt() {
  return (
    <Frame label="右上のメニューをタップ">
      <rect x="0" y="0" width="240" height="40" fill="#334155" />
      <rect x="14" y="11" width="170" height="18" rx="9" fill="#1e293b" />
      <g fill="#fff">
        <circle cx="218" cy="14" r="2.2" />
        <circle cx="218" cy="20" r="2.2" />
        <circle cx="218" cy="26" r="2.2" />
      </g>
      <Pulse cx={218} cy={20} r={12} />
      <rect x="40" y="52" width="160" height="66" rx="8" fill="#1e293b" />
      <text x="120" y="90" textAnchor="middle" fontSize="12" fill="#94a3b8">TrackTalk</text>
    </Frame>
  );
}

export function AndroidAddArt() {
  return (
    <Frame label="「ホーム画面に追加」または「アプリをインストール」を選ぶ">
      <rect x="40" y="6" width="190" height="120" rx="10" fill="#1e293b" />
      <g fontSize="11" fill="#94a3b8">
        <text x="54" y="28">新しいタブ</text>
        <text x="54" y="100">共有…</text>
      </g>
      <rect x="46" y="42" width="178" height="26" rx="7" fill="#2563eb" opacity="0.9" />
      <text x="56" y="59" fontSize="11" fontWeight="700" fill="#fff">ホーム画面に追加</text>
      <rect x="46" y="72" width="178" height="1" fill="#334155" />
    </Frame>
  );
}
