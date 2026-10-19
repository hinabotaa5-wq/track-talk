"use client";

import Link from "next/link";
import { useEffect, useState } from "react";
import { EmptyState, ErrorState } from "@/components/empty-state";
import { InjuryTabs } from "@/components/injury-tabs";
import { useAuth } from "@/components/auth-provider";
import { apiFetch } from "@/lib/api";
import { injuryCaseNeedText, type InjuryCaseGroup, type InjuryCaseStats, type InjuryChoiceStat } from "@/lib/injury-cases";

function StatBars({ title, items }: { title: string; items: InjuryChoiceStat[] }) {
  if (items.length === 0) return null;
  return (
    <section className="space-y-1.5">
      <h3 className="text-sm font-semibold text-slate-100">{title}</h3>
      <ul className="space-y-1.5">
        {items.map((item) => (
          <li key={item.code} className="text-sm">
            <div className="flex justify-between gap-3 text-slate-200">
              <span>{item.label}</span>
              <span className="shrink-0">{item.percent}%（{item.count}人）</span>
            </div>
            <div className="mt-0.5 h-1.5 rounded-full bg-white/10" aria-hidden>
              <div className="h-full rounded-full bg-blue-400" style={{ width: `${item.percent}%` }} />
            </div>
          </li>
        ))}
      </ul>
    </section>
  );
}

function GroupCard({ group, minSample, headingLevel }: { group: InjuryCaseGroup; minSample: number; headingLevel: "h2" | "h3" }) {
  const Heading = headingLevel;
  return (
    <div className="space-y-3 rounded-xl border border-white/20 glass bg-white/10 p-4">
      <div className="flex items-baseline justify-between gap-2">
        <Heading className="text-lg font-semibold">{group.body_part_label}</Heading>
        <span className="text-sm text-slate-200">{group.cases}人が回答</span>
      </div>
      {group.ready ? (
        <>
          <StatBars title="ケガをしたとき" items={group.timings} />
          <StatBars title="そのときの状況" items={group.situations} />
          <StatBars title="どうすればよかったか" items={group.preventions} />
        </>
      ) : (
        <p className="text-sm text-slate-300">{injuryCaseNeedText(group, minSample)}</p>
      )}
    </div>
  );
}

export function DataView() {
  const { session } = useAuth();
  const [stats, setStats] = useState<InjuryCaseStats | null>(null);
  const [failed, setFailed] = useState(false);

  useEffect(() => {
    const controller = new AbortController();
    apiFetch<InjuryCaseStats>("/v1/injury/cases/stats", { signal: controller.signal })
      .then(setStats)
      .catch(() => {
        if (!controller.signal.aborted) setFailed(true);
      });
    return () => controller.abort();
  }, []);

  return (
    <div className="space-y-4">
      <div>
        <h1 className="text-2xl font-bold">ケガ</h1>
        <p className="mt-1 text-sm text-slate-200">ケガをした人の経験を、まだケガをしていない人の予防に</p>
      </div>
      <InjuryTabs current="data" />

      <div className="space-y-2 rounded-xl border border-blue-400/40 bg-blue-950/40 p-4">
        <p className="text-sm text-slate-100">
          ケガをしたときの状況と、そのときどうすればよかったかを、選ぶだけで教えてください。名前・日付・学校などは聞かず、
          集計だけを匿名で見せます（回答が{stats?.min_sample ?? 5}人そろうまで、数は表示しません）。
        </p>
        <Link
          href={session ? "/injury/data/new" : "/login?next=/injury/data/new"}
          className="inline-flex items-center justify-center rounded-full bg-blue-600 px-4.5 py-2 text-sm font-semibold text-white shadow-xs hover:bg-blue-500"
        >
          自分のケガを答える
        </Link>
      </div>

      {failed ? (
        <ErrorState message="データの取得に失敗しました。APIの起動を確認してください。" />
      ) : stats === null ? (
        <p className="text-sm text-slate-200">読み込み中...</p>
      ) : stats.parts.length === 0 ? (
        <EmptyState title="まだ回答がありません" body="回答が集まると、ここにケガごとの集計が出ます。" />
      ) : (
        <>
          <GroupCard group={stats.overall} minSample={stats.min_sample} headingLevel="h2" />
          <ul className="grid gap-3 sm:grid-cols-2">
            {stats.parts.map((group) => (
              <li key={group.body_part}>
                <GroupCard group={group} minSample={stats.min_sample} headingLevel="h3" />
              </li>
            ))}
          </ul>
        </>
      )}

      <p className="text-sm text-slate-300">
        <Link href="/injury/stats" className="underline-offset-2 hover:underline">
          みんなの復帰データ（復帰までの日数の目安）→
        </Link>
      </p>
    </div>
  );
}
