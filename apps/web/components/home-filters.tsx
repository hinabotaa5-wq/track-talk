"use client";

import { useEffect, useState } from "react";
import { useSearchParams } from "next/navigation";
import { homeHref, parseHomeSort, type HomeSort } from "@/lib/home-sort";
import { HomePostButton } from "./home-post-button";

const chips: { sort: HomeSort; label: string }[] = [
  { sort: "new", label: "新しい" },
  { sort: "popular", label: "人気" },
  { sort: "recruitment", label: "募集" },
  { sort: "review", label: "レビュー" },
];

function chipClass(active: boolean) {
  return active
    ? "inline-flex min-h-11 min-w-0 flex-1 items-center justify-center whitespace-nowrap rounded-full border border-white/35 bg-blue-500/75 px-2 text-sm sm:px-4.5 font-semibold text-pure-white shadow-md shadow-blue-950/30 backdrop-blur-xl transition-all duration-200 active:scale-[0.98] cursor-pointer"
    : "inline-flex min-h-11 min-w-0 flex-1 items-center justify-center whitespace-nowrap rounded-full glass bg-white/10 border border-white/20 px-2 text-sm sm:px-4.5 font-medium text-slate-300 transition-all duration-200 hover:bg-white/16 hover:text-white active:scale-[0.98] cursor-pointer";
}

// 縦に流れる一覧(募集・レビュー)では、下へ読み進めるあいだタブを上に引っ込め、上へ戻すと出す。
function useHideOnScrollDown(enabled: boolean) {
  const [hidden, setHidden] = useState(false);
  useEffect(() => {
    if (!enabled) return;
    let last = window.scrollY;
    let frame = 0;
    function update() {
      const y = window.scrollY;
      const delta = y - last;
      // 一番上の近くでは常に出す。指の小さな揺れでは切り替えない。
      if (y < 80) setHidden(false);
      else if (delta > 8) setHidden(true);
      else if (delta < -8) setHidden(false);
      if (y < 80 || Math.abs(delta) > 8) last = y;
    }
    function onScroll() {
      cancelAnimationFrame(frame);
      frame = requestAnimationFrame(update);
    }
    window.addEventListener("scroll", onScroll, { passive: true });
    return () => {
      cancelAnimationFrame(frame);
      window.removeEventListener("scroll", onScroll);
      setHidden(false);
    };
  }, [enabled]);
  return [hidden, setHidden] as const;
}

export function HomeFilters() {
  // チップは history.pushState でタブを切り替える。HomeView は URL を見て、サーバーとやりとりせずに切り替える。
  const sp = useSearchParams();
  const sort = parseHomeSort(sp.get("sort"));
  const q = (sp.get("q") ?? "").trim();

  const [hidden, setHidden] = useHideOnScrollDown(sort === "recruitment" || sort === "review");

  return (
    <div
      data-hidden={hidden || undefined}
      onFocusCapture={() => setHidden(false)}
      className={`px-4 py-3 md:mb-4 md:px-0 motion-safe:transition-[translate,opacity] motion-safe:duration-200 ${hidden ? "pointer-events-none -translate-y-full opacity-0" : ""}`}
    >
      <nav aria-label="投稿の並び替え" className="flex items-center gap-1.5 sm:gap-2">
        <HomePostButton />
        {chips.map((chip) => {
          const href = homeHref(chip.sort, "");
          return (
            <a
              key={chip.sort}
              href={href}
              onClick={(e) => {
                if (e.button !== 0 || e.metaKey || e.ctrlKey || e.shiftKey || e.altKey) return;
                e.preventDefault();
                if (chip.sort !== sort || q) window.history.pushState(null, "", href);
              }}
              className={chipClass(sort === chip.sort)}
              aria-current={sort === chip.sort ? "page" : undefined}
            >
              {chip.label}
            </a>
          );
        })}
      </nav>
    </div>
  );
}
