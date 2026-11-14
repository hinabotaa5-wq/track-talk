"use client";
import { L } from "@/lib/i18n";

import { coachEventLabels } from "@/lib/coach-recruit";
import type { CoachEvent } from "@/lib/types";
import { ChevronLeft, ChevronRight } from "lucide-react";
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

// ドラッグとタップ（選択）を見分ける移動量のしきい値（px）。これを超えたらドラッグ扱いにし、直後の click を選択にしない。
const DRAG_THRESHOLD_PX = 6;
// 矢印ボタン1回で動く量。チップ列の見えている幅に対する割合（#676 仕様1：およそ1画面分）。
const PAGE_SCROLL_RATIO = 0.8;

// 種目チップの列。1列で折り返さず、横にスクロールする。先頭は「すべて」。単一選択。
// スクロールバーは出さず、右にまだ続くときだけ右端を薄くして続きがあることを示す。
// PC幅では左右の矢印ボタンとマウスドラッグでも横スクロールできる（#676）。スマホ幅ではボタンを出さず、スワイプのまま。
export function CoachEventChips({ value, onChange }: { value: CoachEventFilter; onChange: (value: CoachEventFilter) => void }) {
  const scroller = useRef<HTMLDivElement>(null);
  const [moreRight, setMoreRight] = useState(false);
  const [canLeft, setCanLeft] = useState(false);
  const [dragging, setDragging] = useState(false);
  const suppressClick = useRef(false);
  const measure = useCallback(() => {
    const el = scroller.current;
    if (!el) return;
    setMoreRight(el.scrollLeft + el.clientWidth < el.scrollWidth - 1);
    setCanLeft(el.scrollLeft > 1);
  }, []);
  useEffect(() => {
    measure();
    window.addEventListener("resize", measure);
    return () => window.removeEventListener("resize", measure);
  }, [measure]);

  // 選択したチップが列の端で切れていたら、全体が見える位置まで列だけを動かす（#676 仕様3）。
  // すでに全体が見えているチップは動かさない。絞り込み結果には触らない。
  // チップをタップした場合だけでなく、空状態の「すべての種目のコーチを見る」など外から value が変わった場合も通る。
  // チップの位置は「スクロール内容の先頭からの座標」に直してから使う（#697）。offsetLeft は offsetParent
  // （位置指定のある祖先、無ければページ側）基準で、ページ上の配置（main の左右パディングや中央寄せの余白）が
  // 混ざるため使わない。画面上の位置の差に現在の scrollLeft を足せば、配置や画面幅に依存しない。
  useEffect(() => {
    const el = scroller.current;
    if (!el) return;
    const selected = el.querySelector<HTMLElement>(`[data-chip="${value}"]`);
    if (!selected) return;
    const viewRect = el.getBoundingClientRect();
    const chipRect = selected.getBoundingClientRect();
    const viewLeft = el.scrollLeft;
    const viewRight = viewLeft + el.clientWidth;
    const chipLeft = chipRect.left - viewRect.left - el.clientLeft + viewLeft;
    const chipRight = chipLeft + chipRect.width;
    const maxLeft = Math.max(0, el.scrollWidth - el.clientWidth);
    const clamp = (left: number) => Math.min(maxLeft, Math.max(0, left));
    if (chipLeft < viewLeft - 1) {
      el.scrollTo({ left: clamp(chipLeft), behavior: "smooth" });
    } else if (chipRight > viewRight + 1) {
      el.scrollTo({ left: clamp(chipRight - el.clientWidth), behavior: "smooth" });
    }
  }, [value]);

  const scrollPage = useCallback((direction: 1 | -1) => {
    const el = scroller.current;
    if (!el) return;
    el.scrollBy({ left: direction * el.clientWidth * PAGE_SCROLL_RATIO, behavior: "smooth" });
  }, []);

  // マウスドラッグで横スクロール（#676 仕様2）。タッチ（pointerType touch/pen）はブラウザ標準の
  // 横スクロールのまま使い、ここでは触らない。ホイールも何もしない（Shift+ホイールはブラウザ標準のまま）。
  // 掴んだまま列の外で離しても終わるよう、move/up は window で受ける。使う値は閉じ込めた変数だけ。
  const onPointerDown = (e: React.PointerEvent<HTMLDivElement>) => {
    if (e.pointerType !== "mouse" || e.button !== 0) return;
    suppressClick.current = false;
    const startX = e.clientX;
    const startScrollLeft = scroller.current?.scrollLeft ?? 0;
    let moved = false;
    const move = (ev: PointerEvent) => {
      if (ev.pointerType !== "mouse") return;
      const el = scroller.current;
      if (!el) return;
      const dx = ev.clientX - startX;
      if (!moved) {
        if (Math.abs(dx) <= DRAG_THRESHOLD_PX) return;
        moved = true;
        setDragging(true);
      }
      el.scrollLeft = startScrollLeft - dx;
    };
    const end = () => {
      window.removeEventListener("pointermove", move);
      window.removeEventListener("pointerup", end);
      window.removeEventListener("pointercancel", end);
      setDragging(false);
      // ドラッグした直後の click は選択にしない。タップ相当の小さい移動は通す。
      suppressClick.current = moved;
    };
    window.addEventListener("pointermove", move);
    window.addEventListener("pointerup", end);
    window.addEventListener("pointercancel", end);
  };

  const chips: { id: CoachEventFilter; label: string }[] = [
    { id: "all", label: L("すべて") },
    ...coachEventFilterOrder.map((event) => ({ id: event, label: coachEventLabels[event] })),
  ];
  const canScroll = canLeft || moreRight;
  const arrowClass =
    "hidden shrink-0 items-center justify-center rounded-full border border-white/20 bg-white/10 p-1.5 text-slate-300 hover:bg-white/16 hover:text-white disabled:opacity-30 disabled:hover:bg-white/10 disabled:hover:text-slate-300 md:inline-flex";
  return (
    <div className="flex items-center gap-1">
      {canScroll ? (
        <button
          type="button"
          aria-label={L("種目を左へスクロール")}
          disabled={!canLeft}
          onClick={() => scrollPage(-1)}
          className={arrowClass}
        >
          <ChevronLeft className="size-5" aria-hidden />
        </button>
      ) : null}
      <div
        ref={scroller}
        role="radiogroup"
        aria-label={L("種目で絞り込む")}
        data-more-right={moreRight ? "true" : "false"}
        onScroll={measure}
        onPointerDown={onPointerDown}
        onClickCapture={(e) => {
          if (!suppressClick.current) return;
          suppressClick.current = false;
          e.stopPropagation();
          e.preventDefault();
        }}
        className={`coach-event-chips flex min-w-0 flex-1 flex-nowrap gap-2 overflow-x-auto py-0.5 md:cursor-grab ${
          dragging ? "select-none md:cursor-grabbing" : ""
        }`}
      >
        {chips.map((chip) => {
          const selected = value === chip.id;
          return (
            <button
              key={chip.id}
              type="button"
              role="radio"
              aria-checked={selected}
              data-chip={chip.id}
              onClick={() => onChange(chip.id)}
              className={`shrink-0 whitespace-nowrap rounded-full border px-3 py-1 text-sm ${
                selected
                  ? "border-white/35 bg-blue-500/75 font-semibold text-pure-white shadow-md shadow-blue-950/30 backdrop-blur-xl"
                  : "glass border-white/20 bg-white/10 font-medium text-slate-300 hover:bg-white/16 hover:text-white"
              }`}
            >
              {chip.label}
            </button>
          );
        })}
      </div>
      {canScroll ? (
        <button
          type="button"
          aria-label={L("種目を右へスクロール")}
          disabled={!moreRight}
          onClick={() => scrollPage(1)}
          className={arrowClass}
        >
          <ChevronRight className="size-5" aria-hidden />
        </button>
      ) : null}
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
