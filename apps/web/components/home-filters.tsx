"use client";

import { useSearchParams } from "next/navigation";
import { Search } from "lucide-react";
import { homeHref, parseHomeSort, type HomeSort } from "@/lib/home-sort";

const chips: { sort: HomeSort; label: string }[] = [
  { sort: "popular", label: "人気" },
  { sort: "new", label: "新しい順" },
  { sort: "recruitment", label: "募集" },
  { sort: "saved", label: "保存" },
];

function chipClass(active: boolean) {
  return active
    ? "inline-flex min-h-11 items-center rounded-full bg-accent px-4.5 text-sm font-semibold text-accent-foreground shadow-xs transition-all duration-200 active:scale-95 cursor-pointer"
    : "inline-flex min-h-11 items-center rounded-full bg-surface px-4.5 text-sm font-medium text-muted transition-all duration-200 hover:bg-surface-raised hover:text-foreground active:scale-95 cursor-pointer";
}

export function HomeFilters() {
  // Chips switch tabs with history.pushState; HomeView follows the URL without a server round trip.
  const sp = useSearchParams();
  const sort = parseHomeSort(sp.get("sort"));
  const q = (sp.get("q") ?? "").trim();

  return (
    <div className="md:mb-4 md:flex md:items-center md:gap-4">
      <form action="/" className="border-b border-border bg-surface px-4 py-3 md:w-72 md:shrink-0 md:border-0 md:bg-transparent md:p-0">
        {sort !== "popular" ? <input type="hidden" name="sort" value={sort} /> : null}
        <div className="relative">
          <Search
            className="pointer-events-none absolute top-1/2 left-3.5 size-4 -translate-y-1/2 text-muted"
            aria-hidden
          />
          <input
            key={q}
            type="search"
            name="q"
            defaultValue={q}
            placeholder="キーワード"
            aria-label="キーワード"
            className="h-11 w-full rounded-full border border-input-border bg-field pr-4 pl-10 text-base placeholder:text-muted shadow-2xs outline-none transition focus:border-accent focus:ring-2 focus:ring-accent/20"
          />
        </div>
      </form>
      <div className="flex flex-wrap gap-2 px-4 py-3 md:flex-1 md:px-0 md:py-0">
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
      </div>
    </div>
  );
}
