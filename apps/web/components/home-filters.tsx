"use client";

import { useEffect, useRef, useState } from "react";
import { flushSync } from "react-dom";
import { useSearchParams } from "next/navigation";
import { Search, X } from "lucide-react";
import { homeHref, parseHomeSort, type HomeSort } from "@/lib/home-sort";

const chips: { sort: HomeSort; label: string }[] = [
  { sort: "new", label: "新しい" },
  { sort: "popular", label: "人気" },
  { sort: "recruitment", label: "募集" },
  { sort: "review", label: "レビュー" },
];

function chipClass(active: boolean) {
  return active
    ? "inline-flex min-h-11 min-w-0 flex-1 items-center justify-center whitespace-nowrap rounded-full border border-white/35 bg-blue-500/75 px-2 text-sm sm:px-4.5 font-semibold text-white shadow-md shadow-blue-950/30 backdrop-blur-xl transition-all duration-200 active:scale-[0.98] cursor-pointer"
    : "inline-flex min-h-11 min-w-0 flex-1 items-center justify-center whitespace-nowrap rounded-full glass bg-white/10 border border-white/20 px-2 text-sm sm:px-4.5 font-medium text-slate-300 transition-all duration-200 hover:bg-white/16 hover:text-white active:scale-[0.98] cursor-pointer";
}

function KeywordInput({ initial }: { initial: string }) {
  const [value, setValue] = useState(initial);
  const inputRef = useRef<HTMLInputElement>(null);

  useEffect(() => {
    inputRef.current?.focus();
  }, []);

  return (
    <>
      <input
        ref={inputRef}
        type="search"
        name="q"
        value={value}
        onChange={(e) => setValue(e.target.value)}
        placeholder="キーワード"
        aria-label="キーワード"
        className="h-11 w-full rounded-full border border-white/20 glass bg-white/12 pr-10 pl-10 text-base text-white placeholder:text-slate-300 shadow-2xs outline-none transition focus:border-blue-500 focus:ring-2 focus:ring-blue-500/20 [&::-webkit-search-cancel-button]:hidden [&::-webkit-search-cancel-button]:appearance-none"
      />
      {value ? (
        <button
          type="button"
          aria-label="キーワードを消す"
          onClick={() => {
            flushSync(() => setValue(""));
            inputRef.current?.focus();
            // 検索がかかっていれば、それも外す。
            if (initial) inputRef.current?.form?.requestSubmit();
          }}
          className="absolute top-1/2 right-2 z-10 flex size-7 -translate-y-1/2 cursor-pointer items-center justify-center rounded-full text-slate-300 transition hover:bg-white/16 hover:text-white"
        >
          <X className="size-4" aria-hidden />
        </button>
      ) : null}
    </>
  );
}

export function HomeFilters() {
  // チップは history.pushState でタブを切り替える。HomeView は URL を見て、サーバーとやりとりせずに切り替える。
  const sp = useSearchParams();
  const sort = parseHomeSort(sp.get("sort"));
  const q = (sp.get("q") ?? "").trim();

  // 検索URLへの戻る操作でも入力欄を開く。キーワードを消すだけなら入力を続けられる。
  const [searchRequested, setSearchOpen] = useState(false);
  const searchOpen = searchRequested || Boolean(q);
  const searchButtonRef = useRef<HTMLButtonElement>(null);
  const restoreSearchFocus = useRef(false);
  useEffect(() => {
    if (!searchOpen && restoreSearchFocus.current) {
      searchButtonRef.current?.focus();
      restoreSearchFocus.current = false;
    }
  }, [searchOpen]);

  function closeSearch() {
    restoreSearchFocus.current = true;
    setSearchOpen(false);
    if (q) {
      const params = new URLSearchParams(sp.toString());
      params.delete("q");
      params.delete("page");
      const query = params.toString();
      window.history.pushState(null, "", query ? `/?${query}` : "/");
    }
  }

  return (
    <div className="px-4 py-3 md:mb-4 md:px-0">
      {searchOpen ? (
        <div className="flex items-center gap-2" onKeyDown={(event) => {
          if (event.key === "Escape") {
            event.preventDefault();
            closeSearch();
          }
        }}>
          <form action="/" role="search" className="relative min-w-0 flex-1" onSubmit={(event) => {
            event.preventDefault();
            setSearchOpen(true);
            const params = new URLSearchParams();
            new FormData(event.currentTarget).forEach((value, key) => {
              if (typeof value === "string" && value.trim()) params.set(key, value.trim());
            });
            const query = params.toString();
            window.history.pushState(null, "", query ? `/?${query}` : "/");
          }}>
            {sort !== "new" ? <input type="hidden" name="sort" value={sort} /> : null}
            {sort === "review" && sp.get("type") ? <input type="hidden" name="type" value={sp.get("type")!} /> : null}
            <button type="submit" aria-label="検索する" className="absolute top-1/2 left-1 z-10 inline-flex size-9 -translate-y-1/2 items-center justify-center rounded-full text-slate-300 transition hover:bg-white/16 active:scale-[0.98]">
              <Search className="size-4" aria-hidden />
            </button>
            <KeywordInput key={q} initial={q} />
          </form>
          <button type="button" onClick={closeSearch} aria-label="検索を閉じる" className="inline-flex size-11 shrink-0 items-center justify-center rounded-full border border-white/20 bg-white/10 text-slate-300 transition hover:bg-white/16 active:scale-[0.98]">
            <X className="size-5" aria-hidden />
          </button>
        </div>
      ) : (
        <nav aria-label="投稿の並び替えと検索" className="flex items-center gap-1.5 sm:gap-2">
          <button
            ref={searchButtonRef}
            type="button"
            aria-label="キーワード検索を開く"
            aria-expanded={false}
            onClick={() => setSearchOpen(true)}
            className="inline-flex size-11 shrink-0 items-center justify-center rounded-full border border-white/20 glass bg-white/10 text-slate-300 transition hover:bg-white/16 active:scale-[0.98]"
          >
            <Search className="size-5" aria-hidden />
          </button>
          {chips.map((chip) => {
            const href = homeHref(chip.sort, q);
            return (
              <a
                key={chip.sort}
                href={href}
                onClick={(e) => {
                  if (e.button !== 0 || e.metaKey || e.ctrlKey || e.shiftKey || e.altKey) return;
                  e.preventDefault();
                  if (chip.sort !== sort) window.history.pushState(null, "", href);
                }}
                className={chipClass(sort === chip.sort)}
                aria-current={sort === chip.sort ? "page" : undefined}
              >
                {chip.label}
              </a>
            );
          })}
        </nav>
      )}
    </div>
  );
}
