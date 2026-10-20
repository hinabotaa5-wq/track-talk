"use client";

import Link from "next/link";
import { useEffect, useState } from "react";
import { EmptyState, ErrorState } from "@/components/empty-state";
import { InjuryTabs } from "@/components/injury-tabs";
import { Bars, hasItems } from "@/components/injury-public-summary";
import { apiFetch } from "@/lib/api";
import { injuryGroupPath, type InjuryPublicGroup } from "@/lib/injury";

type InjuryData = { min_people: number; overall: InjuryPublicGroup; groups: InjuryPublicGroup[] };

function GroupCard({ group, min, overall }: { group: InjuryPublicGroup; min: number; overall?: boolean }) {
  const Heading = overall ? "h2" : "h3";
  const situation = [
    { title: "ケガの前に変わったこと", tally: group.pre_changes },
    { title: "そのときしていたこと", tally: group.activities },
    { title: "場所", tally: group.places },
    { title: "種目", tally: group.events },
  ];
  const ready = hasItems(group.actions, group.hindsight, ...situation.map((item) => item.tally));
  return (
    <div className="space-y-4 rounded-2xl border border-white/20 glass bg-white/10 p-4">
      <div className="flex items-baseline justify-between gap-2">
        <Heading className="text-lg font-bold">{group.body_part_label}</Heading>
        <span className="text-sm text-slate-200">{group.members}人</span>
      </div>
      {ready ? (
        <>
          {hasItems(...situation.map((item) => item.tally)) ? (
            <section className="space-y-3">
              <h4 className="text-xs font-bold text-slate-300">ケガをしたときの状況</h4>
              {situation.map((item) => (
                <Bars key={item.title} title={item.title} tally={item.tally} />
              ))}
            </section>
          ) : null}
          {hasItems(group.hindsight) ? (
            <section className="space-y-3">
              <h4 className="text-xs font-bold text-slate-300">いま思うと、どうすればよかったか</h4>
              <Bars title="選んだ人の割合" tally={group.hindsight} />
            </section>
          ) : null}
          {hasItems(group.actions) ? (
            <section className="space-y-3">
              <h4 className="text-xs font-bold text-slate-300">復帰までにやったこと</h4>
              <Bars title="やった人の割合" tally={group.actions} />
            </section>
          ) : null}
        </>
      ) : (
        <p className="text-sm text-slate-400">{min}人そろうと表示</p>
      )}
      {!overall ? (
        <Link href={injuryGroupPath(group.body_part)} className="block text-sm text-blue-300 underline-offset-2 hover:underline">
          このグループを見る →
        </Link>
      ) : null}
    </div>
  );
}

// 「データ」タブ。ケガ共有のグループで集まった回答を、全体とケガごとに集計して見せるだけで、ここでは何も集めない。
export function DataView() {
  const [data, setData] = useState<InjuryData | null>(null);
  const [failed, setFailed] = useState(false);

  useEffect(() => {
    const controller = new AbortController();
    apiFetch<InjuryData>("/v1/injury/data", { signal: controller.signal })
      .then(setData)
      .catch(() => {
        if (!controller.signal.aborted) setFailed(true);
      });
    return () => controller.abort();
  }, []);

  return (
    <div className="space-y-4">
      <h1 className="sr-only">ケガ</h1>
      <p className="text-sm text-slate-200">ケガをした人の経験を、まだケガをしていない人の予防に</p>
      <InjuryTabs current="data" />

      {failed ? (
        <ErrorState message="データの取得に失敗しました。APIの起動を確認してください。" />
      ) : data === null ? (
        <p className="text-sm text-slate-200">読み込み中...</p>
      ) : data.groups.length === 0 ? (
        <EmptyState title="まだデータがありません" body="共有のグループに参加する人が増えると、ここに集計が出ます。" />
      ) : (
        <>
          <GroupCard group={data.overall} min={data.min_people} overall />
          <ul className="grid gap-3 sm:grid-cols-2">
            {data.groups.map((group) => (
              <li key={group.body_part}>
                <GroupCard group={group} min={data.min_people} />
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
