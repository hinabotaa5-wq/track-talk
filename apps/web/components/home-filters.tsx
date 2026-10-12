"use client";

import Link from "next/link";
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
    ? "inline-flex min-h-11 items-center rounded-full bg-blue-600 px-4 text-sm font-medium text-white"
    : "inline-flex min-h-11 items-center rounded-full bg-zinc-100 px-4 text-sm text-zinc-700";
}

export function HomeFilters() {
  // Read from the URL so the active chip switches on tap, before the server responds.
  const sp = useSearchParams();
  const sort = parseHomeSort(sp.get("sort"));
  const q = (sp.get("q") ?? "").trim();

  return (
    <div className="md:mb-4 md:flex md:items-center md:gap-4">
      <form action="/" className="border-b border-zinc-200 bg-white px-4 py-3 md:w-72 md:shrink-0 md:border-0 md:bg-transparent md:p-0">
        {sort !== "popular" ? <input type="hidden" name="sort" value={sort} /> : null}
        <div className="relative">
          <Search
            className="pointer-events-none absolute top-1/2 left-3 size-4 -translate-y-1/2 text-zinc-400"
            aria-hidden
          />
          <input
            key={q}
            type="search"
            name="q"
            defaultValue={q}
            placeholder="キーワード"
            aria-label="キーワード"
            className="h-11 w-full rounded-full border border-zinc-300 bg-white pr-4 pl-10 text-base outline-none focus:border-blue-500"
          />
        </div>
      </form>
      <div className="flex flex-wrap gap-2 px-4 py-3 md:flex-1 md:px-0 md:py-0">
        {chips.map((chip) => (
          <Link
            key={chip.sort}
            href={homeHref(chip.sort, q)}
            className={chipClass(sort === chip.sort)}
            aria-current={sort === chip.sort ? "page" : undefined}
          >
            {chip.label}
          </Link>
        ))}
      </div>
    </div>
  );
}
