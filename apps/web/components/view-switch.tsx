"use client";

import { motion } from "motion/react";
import type { MouseEvent } from "react";

// 2つの表示を行き来する丸いスイッチ。2つ並べて切り替えるところは、すべてこれを使う。
// (大会タブ・ケガタブ・メモの感覚/記録・自分の投稿の並び順・レビューの種類・管理者画面の商品の種類)
// href を渡すと、検索エンジンや新しいタブでも開けるようにリンクで描く。
export function ViewSwitch<T extends string>({
  label,
  layoutId,
  views,
  current,
  onSelect,
  href,
}: {
  label: string;
  layoutId: string;
  views: readonly { id: T; label: string }[];
  current: T;
  onSelect: (next: T) => void;
  href?: (id: T) => string;
}) {
  return (
    <div role="group" aria-label={label} className="grid grid-cols-2 gap-1 rounded-full border border-white/25 glass bg-white/10 p-1 md:max-w-sm">
      {views.map((item) => {
        const active = item.id === current;
        const className = `relative flex h-10 items-center justify-center rounded-full text-sm font-semibold transition-colors duration-200 active:scale-95 ${
          active ? "text-white" : "text-slate-200 hover:text-white"
        }`;
        const inner = (
          <>
            {active ? (
              <motion.span
                layoutId={layoutId}
                className="absolute inset-0 rounded-full bg-white/25"
                transition={{ type: "spring", stiffness: 500, damping: 40 }}
                aria-hidden
              />
            ) : null}
            <span className="relative">{item.label}</span>
          </>
        );
        if (href) {
          return (
            <a
              key={item.id}
              href={href(item.id)}
              aria-current={active ? "page" : undefined}
              onClick={(e: MouseEvent) => {
                // 新しいタブで開くなどの操作はブラウザに任せる。
                if (e.button !== 0 || e.metaKey || e.ctrlKey || e.shiftKey || e.altKey) return;
                e.preventDefault();
                onSelect(item.id);
              }}
              className={className}
            >
              {inner}
            </a>
          );
        }
        return (
          <button key={item.id} type="button" aria-pressed={active} onClick={() => onSelect(item.id)} className={className}>
            {inner}
          </button>
        );
      })}
    </div>
  );
}
