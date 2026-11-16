import { L } from "@/lib/i18n";
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
    <Frame label={L("Safari のアイコンをタップして開く")}>
      <rect x="20" y="22" width="200" height="30" rx="15" fill="#334155" />
      <text x="120" y="42" textAnchor="middle" fontSize="12" fill="#cbd5e1">{L("tracktalk の URL")}</text>
      <circle cx="120" cy="92" r="22" fill="#f8fafc" />
      <circle cx="120" cy="92" r="17" fill="none" stroke="#2563eb" strokeWidth="3" />
      <path d="M120 80l5 12-5 12-5-12z" fill="#ef4444" />
      <Pulse cx={120} cy={92} r={26} />
    </Frame>
  );
}

export function IosShareButtonArt() {
  return (
    <Frame label={L("アドレスバー左の「≡」をタップし、メニューの一番上の「共有」をタップ")}>
      <rect x="52" y="6" width="136" height="58" rx="10" fill="#1e293b" />
      <rect x="58" y="12" width="124" height="22" rx="6" fill="#2563eb" opacity="0.9" />
      <ShareGlyph x={72} y={22} color="#fff" />
      <text x="86" y="27" fontSize="11" fontWeight="700" fill="#fff">{L("共有")}</text>
      <text x="64" y="52" fontSize="10" fill="#94a3b8">{L("ブックマークに追加")}</text>
      <path d="M30 86v10" stroke={accent} strokeWidth="2.5" strokeLinecap="round" />
      <rect x="14" y="96" width="212" height="26" rx="13" fill="#334155" />
      <text x="30" y="114" textAnchor="middle" fontSize="14" fill="#fff">≡</text>
      <text x="122" y="113" textAnchor="middle" fontSize="10" fill="#cbd5e1">{L("tracktalk の URL")}</text>
      <text x="204" y="114" textAnchor="middle" fontSize="13" fill="#94a3b8">↻</text>
      <Pulse cx={30} cy={109} r={13} />
    </Frame>
  );
}

export function IosAddToHomeArt() {
  return (
    <Frame label={L("メニューから「ホーム画面に追加」を選ぶ")}>
      <rect x="30" y="8" width="180" height="116" rx="12" fill="#1e293b" />
      <g fontSize="10" fill="#94a3b8">
        <text x="46" y="28">{L("ブックマークの追加先…")}</text>
        <text x="46" y="45">{L("お気に入りに追加")}</text>
        <text x="46" y="62">{L("ページを検索")}</text>
        <text x="46" y="101">{L("マークアップ")}</text>
        <text x="46" y="118">{L("プリント")}</text>
      </g>
      <rect x="36" y="68" width="168" height="24" rx="7" fill="#2563eb" opacity="0.9" />
      <text x="46" y="84" fontSize="11" fontWeight="700" fill="#fff">{L("ホーム画面に追加")}</text>
      <rect x="176" y="73" width="14" height="14" rx="3" fill="none" stroke="#fff" strokeWidth="1.8" />
      <path d="M183 76v8M179 80h8" stroke="#fff" strokeWidth="1.8" strokeLinecap="round" />
      <path d="M212 80h-14" stroke={accent} strokeWidth="2.5" strokeLinecap="round" />
      <path d="M200 75l-6 5 6 5" fill="none" stroke={accent} strokeWidth="2.5" strokeLinecap="round" strokeLinejoin="round" />
    </Frame>
  );
}

export function IosConfirmArt() {
  return (
    <Frame label={L("右上の「追加」をタップ")}>
      <rect x="20" y="8" width="200" height="116" rx="12" fill="#1e293b" />
      <text x="30" y="30" fontSize="8" fill="#94a3b8">{L("キャンセル")}</text>
      <text x="112" y="30" textAnchor="middle" fontSize="8.5" fontWeight="700" fill="#e2e8f0">{L("ホーム画面に追加")}</text>
      <rect x="182" y="17" width="28" height="18" rx="9" fill="#2563eb" />
      <text x="196" y="30" textAnchor="middle" fontSize="10" fontWeight="700" fill="#fff">{L("追加")}</text>
      <Pulse cx={196} cy={26} r={17} />
      <rect x="34" y="50" width="44" height="44" rx="10" fill="#ffffff" />
      <TrackLanesMark x="34" y="50" width="44" height="44" />
      <text x="90" y="68" fontSize="12" fontWeight="700" fill="#e2e8f0">TrackTalk</text>
      <text x="90" y="84" fontSize="9" fill="#94a3b8">{L("tracktalk のアドレス")}</text>
    </Frame>
  );
}

export function AndroidMenuArt() {
  return (
    <Frame label={L("右上のメニューをタップ")}>
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
    <Frame label={L("「ホーム画面に追加」または「アプリをインストール」を選ぶ")}>
      <rect x="40" y="6" width="190" height="120" rx="10" fill="#1e293b" />
      <g fontSize="11" fill="#94a3b8">
        <text x="54" y="28">{L("新しいタブ")}</text>
        <text x="54" y="100">{L("共有…")}</text>
      </g>
      <rect x="46" y="42" width="178" height="26" rx="7" fill="#2563eb" opacity="0.9" />
      <text x="56" y="59" fontSize="11" fontWeight="700" fill="#fff">{L("ホーム画面に追加")}</text>
      <rect x="46" y="72" width="178" height="1" fill="#334155" />
    </Frame>
  );
}
