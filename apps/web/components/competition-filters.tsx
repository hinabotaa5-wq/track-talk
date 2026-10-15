"use client";

import { useRouter } from "next/navigation";
import { regionalCompetitionsPath, saveCompetitionPrefecture } from "@/lib/competition-prefecture";
import { PREFECTURES } from "@/lib/prefectures";

export function CompetitionFilters({
  month,
  prefecture,
}: {
  month: number;
  prefecture: string;
}) {
  const router = useRouter();

  function apply(nextMonth: string, nextPrefecture: string) {
    const params = new URLSearchParams();
    params.set("month", nextMonth);
    if (nextPrefecture) {
      saveCompetitionPrefecture(nextPrefecture);
      params.set("prefecture", nextPrefecture);
    }
    router.push(`${regionalCompetitionsPath}?${params.toString()}`);
  }

  return (
    <div className="grid grid-cols-2 gap-3 md:max-w-xl">
      <label className="block text-sm">
        <span className="mb-1 block text-slate-200">月</span>
        <select
          name="month"
          value={String(month)}
          onChange={(e) => apply(e.target.value, prefecture)}
          className="h-11 w-full rounded-md border border-white/25 glass bg-white/10 px-3 text-base text-slate-100 outline-none focus:border-blue-400"
        >
          {Array.from({ length: 12 }, (_, i) => i + 1).map((m) => (
            <option key={m} value={m}>
              {m}月
            </option>
          ))}
        </select>
      </label>
      <label className="block text-sm">
        <span className="mb-1 block text-slate-200">都道府県</span>
        <select
          name="prefecture"
          value={prefecture}
          onChange={(e) => apply(String(month), e.target.value)}
          className="h-11 w-full rounded-md border border-white/25 glass bg-white/10 px-3 text-base text-slate-100 outline-none focus:border-blue-400"
        >
          {prefecture ? null : (
            <option value="" disabled>
              選んでください
            </option>
          )}
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
