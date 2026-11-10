"use client";
import { L } from "@/lib/i18n";

import { coachEventLabels } from "@/lib/coach-recruit";
import type { CoachEvent } from "@/lib/types";
import { useCallback, useEffect, useRef, useState } from "react";

// コーチ一覧の種目の絞り込み（本体 #507）。動画添削の指名一覧とサポートプランの一覧で同じ部品を使う。
// 種目の値と表示名は担当種目のマスタ（coachEventLabels。API の appEvents と同じ一覧）をそのまま使い、
// ここでは並べる順だけを持つ。近い種目が隣り合う区分順（短距離→中長距離→障害→ハードル→跳躍→投てき→混成→その他）。
// マスタに種目を足したら、テストがここへの追加漏れを知らせる。
export const coachEventFilterOrder: readonly CoachEvent[] = [
  "100m",
  "200m",
  "400m",
  "800m",
  "1500m",
  "5000m",
  "10000m",
  "3000msc",
  "100mh",
  "110mh",
  "400mh",
  "high-jump",
  "pole-vault",
  "long-jump",
  "triple-jump",
  "shot-put",
  "discus",
  "hammer",
  "javelin",
  "decathlon",
  "heptathlon",
  "race-walk",
  "relay",
];

export type CoachEventFilter = "all" | CoachEvent;

// コーチ単位の絞り込み。コーチが宣言した担当種目に含まれていれば出す（複数種目のコーチはそれぞれの種目で出る）。
// 並び順は受け取った順（API の公平なローテーション）のまま変えない。
export function filterCoachesByEvent<T extends { events: readonly CoachEvent[] }>(coaches: readonly T[], event: CoachEventFilter): T[] {
  if (event === "all") return [...coaches];
  return coaches.filter((coach) => coach.events.includes(event));
}

// 種目チップの列。1列で折り返さず、横にスクロールする。先頭は「すべて」。単一選択。
// スクロールバーは出さず、右にまだ続くときだけ右端を薄くして続きがあることを示す。
export function CoachEventChips({ value, onChange }: { value: CoachEventFilter; onChange: (value: CoachEventFilter) => void }) {
  const scroller = useRef<HTMLDivElement>(null);
  const [moreRight, setMoreRight] = useState(false);
  const measure = useCallback(() => {
    const el = scroller.current;
    if (!el) return;
    setMoreRight(el.scrollLeft + el.clientWidth < el.scrollWidth - 1);
  }, []);
  useEffect(() => {
    measure();
    window.addEventListener("resize", measure);
    return () => window.removeEventListener("resize", measure);
  }, [measure]);

  const chips: { id: CoachEventFilter; label: string }[] = [
    { id: "all", label: L("すべて") },
    ...coachEventFilterOrder.map((event) => ({ id: event, label: coachEventLabels[event] })),
  ];
  return (
    <div
      ref={scroller}
      role="radiogroup"
      aria-label={L("種目で絞り込む")}
      data-more-right={moreRight ? "true" : "false"}
      onScroll={measure}
      className="coach-event-chips flex flex-nowrap gap-2 overflow-x-auto py-0.5"
    >
      {chips.map((chip) => {
        const selected = value === chip.id;
        return (
          <button
            key={chip.id}
            type="button"
            role="radio"
            aria-checked={selected}
            onClick={() => onChange(chip.id)}
            className={`shrink-0 whitespace-nowrap rounded-full border px-3 py-1 text-sm ${
              selected ? "border-blue-600 bg-blue-600 font-semibold text-white" : "border-white/30 bg-white/10 text-slate-200"
            }`}
          >
            {selected ? <span aria-hidden="true" className="mr-1">✓</span> : null}
            {chip.label}
          </button>
        );
      })}
    </div>
  );
}

// 選んだ種目のコーチがいないとき。開始直後はふつうに起こるので、空白にせず「すべて」へ戻る導線を出す。
export function CoachEventEmpty({ onReset }: { onReset: () => void }) {
  return (
    <div className="glass space-y-2 rounded-xl border border-white/20 bg-white/10 p-4 text-center text-sm">
      <p className="text-slate-200">{L("この種目のコーチはまだいません")}</p>
      <button type="button" onClick={onReset} className="font-semibold text-blue-300 underline underline-offset-2">{L("すべての種目のコーチを見る")}</button>
    </div>
  );
}
