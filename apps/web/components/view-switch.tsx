"use client";

import { motion } from "motion/react";
import { useEffect, useState, type MouseEvent } from "react";

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
  initialPillStatic = false,
}: {
  label: string;
  layoutId: string;
  views: readonly { id: T; label: string }[];
  current: T;
  onSelect: (next: T) => void;
  href?: (id: T) => string;
  // 切り替え自体が後から現れる画面用。最初の表示だけピルを最終位置に静止させ、
  // 利用者が押したときの左右への滑りは残す。指定しない画面は従来どおり。
  initialPillStatic?: boolean;
}) {
  const [pillSettled, setPillSettled] = useState(false);
  useEffect(() => {
    if (!initialPillStatic) return;
    const frame = requestAnimationFrame(() => setPillSettled(true));
    return () => cancelAnimationFrame(frame);
  }, [initialPillStatic]);
  return (
    <div role="group" aria-label={label} className="grid grid-cols-2 gap-1 rounded-full border border-white/25 glass bg-white/10 p-1 md:max-w-sm">
      {views.map((item) => {
        const active = item.id === current;
        const className = `relative flex h-10 items-center justify-center rounded-full text-sm font-semibold transition-colors duration-200 active:scale-[0.98] ${
          active ? "text-pure-white" : "text-slate-200 hover:text-blue-300"
        }`;
        const inner = (
          <>
            {active ? (
              initialPillStatic && !pillSettled ? (
                <span className="absolute inset-0 rounded-full bg-blue-600" aria-hidden />
              ) : (
                <motion.span
                  layoutId={layoutId}
                  className="absolute inset-0 rounded-full bg-blue-600"
                  transition={{ type: "spring", stiffness: 500, damping: 40 }}
                  aria-hidden
                />
              )
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
