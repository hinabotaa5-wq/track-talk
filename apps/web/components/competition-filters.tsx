"use client";

import { useRouter } from "next/navigation";
import { saveCompetitionPrefecture } from "@/lib/competition-prefecture";
import { INTERNATIONAL, PREFECTURES } from "@/lib/prefectures";

export function CompetitionFilters({
  month,
  prefecture,
}: {
  month: number;
  prefecture: string;
}) {
  const router = useRouter();

  function apply(nextMonth: string, nextPrefecture: string) {
    const code = nextPrefecture || INTERNATIONAL;
    saveCompetitionPrefecture(code);
    const params = new URLSearchParams();
    params.set("month", nextMonth);
    params.set("prefecture", code);
    router.push(`/competitions?${params.toString()}`);
  }

  return (
    <div className="grid grid-cols-2 gap-3 md:max-w-xl">
      <label className="block text-sm">
        <span className="mb-1 block text-muted">月</span>
        <select
          name="month"
          value={String(month)}
          onChange={(e) => apply(e.target.value, prefecture)}
          className="h-11 w-full rounded-md border border-input-border bg-field px-3 text-base outline-none focus:border-accent"
        >
          {Array.from({ length: 12 }, (_, i) => i + 1).map((m) => (
            <option key={m} value={m}>
              {m}月
            </option>
          ))}
        </select>
      </label>
      <label className="block text-sm">
        <span className="mb-1 block text-muted">地域</span>
        <select
          name="prefecture"
          value={prefecture}
          onChange={(e) => apply(String(month), e.target.value)}
          className="h-11 w-full rounded-md border border-input-border bg-field px-3 text-base outline-none focus:border-accent"
        >
          <option value={INTERNATIONAL}>国際</option>
          {PREFECTURES.map((p) => (
            <option key={p.code} value={p.code}>
              {p.name}
            </option>
          ))}
        </select>
      </label>
    </div>
  );
}
