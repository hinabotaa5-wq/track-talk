"use client";

import Link from "next/link";
import { useEffect, useState } from "react";
import { EmptyState, ErrorState } from "@/components/empty-state";
import { InjuryMeCards } from "@/components/injury-me-cards";
import { useAuth } from "@/components/auth-provider";
import { apiFetch } from "@/lib/api";
import { injuryGroupPath, sortInjuryGroups, type InjuryGroupSummary } from "@/lib/injury";

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

  const sorted = groups ? sortInjuryGroups(groups) : null;

  return (
    <div className="space-y-4">
      <p className="text-sm text-slate-200">同じ怪我をしている人たちと気持ちを共有して復帰を目指そう！</p>
      <InjuryMeCards token={token} />

      {failed ? (
        <ErrorState message="グループの取得に失敗しました。APIの起動を確認してください。" />
      ) : sorted === null ? (
        <div role="status" aria-busy="true" className="grid grid-cols-2 gap-3">
          <p className="sr-only">グループを読み込んでいます</p>
          {Array.from({ length: 6 }, (_, i) => (
            <div key={i} className="h-24 animate-pulse rounded-2xl glass bg-white/12" />
          ))}
        </div>
      ) : sorted.length === 0 ? (
        <EmptyState title="グループがありません" body="しばらくしてからもう一度開いてください。" />
      ) : (
        <ul className="grid grid-cols-2 gap-3">
          {sorted.map((group) => (
            <li key={group.body_part}>
              <Link
                href={injuryGroupPath(group.body_part)}
                className={`block h-full rounded-2xl border p-3.5 transition active:scale-[0.98] ${
                  group.me ? "border-blue-400 bg-blue-950/50 hover:bg-blue-950/70" : "border-white/20 glass bg-white/10 hover:bg-white/16"
                }`}
              >
                <span className="block text-base font-bold">{group.body_part_label}</span>
                <span className="mt-1 block text-sm text-slate-200">{group.members}人</span>
                <span className={`mt-2 block text-xs font-semibold ${group.me ? "text-blue-200" : "text-slate-300"}`}>
                  {group.me ? `参加中 · ${group.me.stage_label}` : "グループに入る"}
                </span>
              </Link>
            </li>
          ))}
        </ul>
      )}

    </div>
  );
}
