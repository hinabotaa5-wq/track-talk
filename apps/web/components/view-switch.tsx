"use client";

import { motion } from "motion/react";

// 2つの表示を行き来する丸いスイッチ。大会タブの「国内主要大会 / 国際大会」とケガタブの「共有 / データ」で使う。
export function ViewSwitch<T extends string>({
  label,
  layoutId,
  views,
  current,
  onSelect,
}: {
  label: string;
  layoutId: string;
  views: readonly { id: T; label: string }[];
  current: T;
  onSelect: (next: T) => void;
}) {
  return (
    <div role="group" aria-label={label} className="grid grid-cols-2 gap-1 rounded-full border border-white/25 glass bg-white/10 p-1 md:max-w-sm">
      {views.map((item) => {
        const active = item.id === current;
        return (
          <button
            key={item.id}
            type="button"
            aria-pressed={active}
            onClick={() => onSelect(item.id)}
            className={`relative h-10 rounded-full text-sm font-semibold transition-colors duration-200 active:scale-95 ${
              active ? "text-white" : "text-slate-200 hover:text-white"
            }`}
          >
            {active ? (
              <motion.span
                layoutId={layoutId}
                className="absolute inset-0 rounded-full bg-white/25"
                transition={{ type: "spring", stiffness: 500, damping: 40 }}
                aria-hidden
              />
            ) : null}
            <span className="relative">{item.label}</span>
          </button>
        );
      })}
    </div>
  );
}

