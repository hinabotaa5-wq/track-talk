"use client";

import Link from "next/link";
import { useEffect, useState } from "react";
import { EmptyState, ErrorState } from "@/components/empty-state";
import { InjuryMeCards } from "@/components/injury-me-cards";
import { InjuryPartIcon } from "@/components/injury-body-figure";
import { InjuryBodyMap } from "@/components/injury-body-map";
import { useAuth } from "@/components/auth-provider";
import { apiFetch } from "@/lib/api";
import {
  defaultInjuryRegion,
  injuryGroupPath,
  injuryGroupsByRegion,
  type InjuryGroupSummary,
  type InjuryRegion,
} from "@/lib/injury";

// 「共有」タブ。ケガのグループの一覧。
export function InjuryView() {
  const { token, loading } = useAuth();
  const [groups, setGroups] = useState<InjuryGroupSummary[] | null>(null);
  const [failed, setFailed] = useState(false);

  useEffect(() => {
    if (loading) return;
    const controller = new AbortController();
    apiFetch<{ groups: InjuryGroupSummary[] }>("/v1/injury/groups", { token, signal: controller.signal })
      .then((result) => {
        setGroups(result.groups ?? []);
        setFailed(false);
      })
      .catch(() => {
        if (!controller.signal.aborted) setFailed(true);
      });
    return () => controller.abort();
  }, [token, loading]);

  return (
    <div className="space-y-4">
      <p className="text-sm text-slate-200">同じ怪我をしている人たちと気持ちを共有して復帰を目指そう！</p>
      <InjuryMeCards token={token} />

      {failed ? (
        <ErrorState message="グループの取得に失敗しました。APIの起動を確認してください。" />
      ) : groups === null ? (
        <div role="status" aria-busy="true" className="space-y-3">
          <p className="sr-only">グループを読み込んでいます</p>
          <div className="mx-auto h-60 w-64 animate-pulse rounded-2xl glass bg-white/12" />
          {Array.from({ length: 3 }, (_, i) => (
            <div key={i} className="h-16 animate-pulse rounded-2xl glass bg-white/12" />
          ))}
        </div>
      ) : groups.length === 0 ? (
        <EmptyState title="グループがありません" body="しばらくしてからもう一度開いてください。" />
      ) : (
        <InjuryGroupPicker groups={groups} />
      )}
    </div>
  );
}

// 体のイラストでまとまりを選び、その中のケガのグループを並べる。
export function InjuryGroupPicker({ groups }: { groups: InjuryGroupSummary[] }) {
  const [picked, setPicked] = useState<InjuryRegion | null>(null);
  const regions = injuryGroupsByRegion(groups);
  const selected = picked ?? defaultInjuryRegion(groups);
  const members = Object.fromEntries(regions.map((region) => [region.value, region.members])) as Record<InjuryRegion, number>;
  const current = regions.find((region) => region.value === selected) ?? regions[0];

  return (
    <section aria-label="体の場所からグループを選ぶ" className="space-y-3">
      <p className="text-center text-sm text-slate-300">痛いところをタップしてね</p>
      <InjuryBodyMap members={members} selected={selected} onSelect={setPicked} />
      <div role="group" aria-label="体の場所" className="flex flex-wrap gap-2">
        {regions.map((region) => (
          <button
            key={region.value}
            type="button"
            aria-pressed={region.value === selected}
            onClick={() => setPicked(region.value)}
            className={`rounded-full border px-3 py-1 text-sm font-semibold transition active:scale-[0.97] ${
              region.value === selected ? "border-blue-400 bg-blue-400 text-slate-950" : "border-white/20 glass bg-white/10 text-slate-200 hover:bg-white/16"
            }`}
          >
            {region.label}
          </button>
        ))}
      </div>
      {current.groups.length === 0 ? (
        <p className="py-4 text-center text-sm text-slate-300">この場所のグループはまだありません。</p>
      ) : (
        <ul className="space-y-2" aria-label={`${current.label}のグループ`}>
          {current.groups.map((group) => (
            <li key={group.body_part}>
              <Link
                href={injuryGroupPath(group.body_part)}
                className={`flex items-center gap-3 rounded-2xl border py-2 pl-2 pr-4 transition active:scale-[0.98] ${
                  group.me ? "border-blue-400 bg-blue-950/50 hover:bg-blue-950/70" : "border-white/20 glass bg-white/10 hover:bg-white/16"
                }`}
              >
                <InjuryPartIcon part={group.body_part} className="h-16 w-9 shrink-0" />
                <span className="min-w-0 flex-1 text-base font-bold">{group.body_part_label}</span>
                <span className="shrink-0 text-right">
                  <span className="block text-sm text-slate-200">{group.members}人</span>
                  <span className={`block text-xs font-semibold ${group.me ? "text-blue-200" : "text-slate-300"}`}>
                    {group.me ? `参加中 · ${group.me.stage_label}` : "グループに入る"}
                  </span>
                </span>
              </Link>
            </li>
          ))}
        </ul>
      )}
    </section>
  );
}
