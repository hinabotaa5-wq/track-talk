"use client";

import Link from "next/link";
import { useEffect, useState } from "react";
import { EmptyState, ErrorState } from "@/components/empty-state";
import { Bars, hasItems } from "@/components/injury-public-summary";
import { apiFetch } from "@/lib/api";
import { ChevronDown } from "lucide-react";
import { injuryGroupPath, injuryPercent, type InjuryPublicGroup, type InjuryTally } from "@/lib/injury";

type InjuryData = { min_people: number; overall: InjuryPublicGroup; groups: InjuryPublicGroup[] };

// 質問ごとの見出し。何を聞いた答えなのかが一目でわかるよう、質問の形にする。
function questionsOf(group: InjuryPublicGroup) {
  return [
    { title: "ケガの前に、何か変わった？", short: "ケガの前の変化", tally: group.pre_changes },
    { title: "そのとき何をしていた？", short: "していたこと", tally: group.activities },
    { title: "どこでケガをした？", short: "場所", tally: group.places },
    { title: "種目は？", short: "種目", tally: group.events },
    { title: "いま思うと、どうすればよかった？", short: "どうすればよかったか", tally: group.hindsight },
    { title: "復帰までに何をした？", short: "復帰までにやったこと", tally: group.actions },
  ];
}

function topItem(tally: InjuryTally) {
  return [...tally.items].sort((a, b) => b.people - a.people)[0];
}

// いちばん多かった答えを、質問ごとに1行で並べる。棒グラフを読む前に、まずここで要点がわかる。
function Highlights({ group }: { group: InjuryPublicGroup }) {
  const rows = questionsOf(group)
    .map((q) => ({ ...q, item: topItem(q.tally) }))
    .filter((row) => row.item);
  if (rows.length === 0) return null;
  return (
    <dl className="grid grid-cols-2 gap-2">
      {rows.map((row) => (
        <div key={row.short} className="rounded-xl border border-white/15 bg-white/8 p-2.5">
          <dt className="text-xs text-slate-300">{row.short}でいちばん多い</dt>
          <dd className="mt-0.5 text-sm font-bold leading-snug text-slate-50">{row.item.label}</dd>
          <dd className="text-xs tabular-nums text-sky-200">
            {injuryPercent(row.item.people, row.tally.people)}%({row.tally.people}人中{row.item.people}人)
          </dd>
        </div>
      ))}
    </dl>
  );
}

function Details({ group }: { group: InjuryPublicGroup }) {
  return (
    <div className="space-y-4">
      {questionsOf(group).map((q) => (
        <Bars key={q.short} title={q.title} tally={q.tally} top={3} />
      ))}
    </div>
  );
}

function hasData(group: InjuryPublicGroup) {
  return hasItems(...questionsOf(group).map((q) => q.tally));
}

// 全体のデータ。要点のあとに、質問ごとの上位3つを出す。
function OverallCard({ group, min }: { group: InjuryPublicGroup; min: number }) {
  return (
    <section className="space-y-4 rounded-2xl border border-white/20 glass bg-white/10 p-4">
      <div className="flex items-baseline justify-between gap-3">
        <h2 className="text-lg font-bold">{group.body_part_label}</h2>
        <span className="text-sm text-slate-300">{group.members}人</span>
      </div>
      {hasData(group) ? (
        <>
          <Highlights group={group} />
          <Details group={group} />
        </>
      ) : (
        <p className="text-sm text-slate-400">{min}人そろうと表示</p>
      )}
    </section>
  );
}

// ケガごとのデータ。並べると長くなるので、閉じた状態で要点だけ見せ、開くと詳しく見られる。
function GroupCard({ group, min }: { group: InjuryPublicGroup; min: number }) {
  const ready = hasData(group);
  const first = topItem(group.pre_changes) ?? topItem(group.activities);
  return (
    <details className="group rounded-2xl border border-white/20 glass bg-white/10 p-4">
      <summary className="flex cursor-pointer list-none items-start justify-between gap-3">
        <span className="min-w-0">
          <span className="block font-bold">{group.body_part_label}</span>
          <span className="mt-0.5 block text-xs text-slate-300">
            {ready ? (first ? `多かったのは「${first.label}」` : "データを見る") : `${min}人そろうと表示`}
          </span>
        </span>
        <span className="flex shrink-0 items-center gap-1 text-sm text-slate-300">
          {group.members}人
          <ChevronDown className="size-4 transition group-open:rotate-180" aria-hidden />
        </span>
      </summary>
      <div className="mt-4 space-y-4">
        {ready ? (
          <>
            <Highlights group={group} />
            <Details group={group} />
          </>
        ) : null}
        <Link href={injuryGroupPath(group.body_part)} className="block text-sm text-blue-300 underline-offset-2 hover:underline">
          このグループを見る →
        </Link>
      </div>
    </details>
  );
}

// 「データ」タブ。共有のグループで集まった回答を、全体とケガごとに集計して見せるだけで、ここでは何も集めない。
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
      <div className="space-y-1">
        <p className="text-sm text-slate-200">ケガをした人の経験を、まだケガをしていない人の予防に</p>
        <p className="text-xs text-slate-400">
          グループに入るときの質問への答えを集計しています。%は、その質問に答えた人のうち、その答えを選んだ人の割合です(いくつでも選べる質問は、合計が100%をこえます)。
        </p>
      </div>

      {failed ? (
        <ErrorState message="データの取得に失敗しました。APIの起動を確認してください。" />
      ) : data === null ? (
        <p className="text-sm text-slate-200">読み込み中...</p>
      ) : data.groups.length === 0 ? (
        <EmptyState title="まだデータがありません" body="共有のグループに参加する人が増えると、ここに集計が出ます。" />
      ) : (
        <>
          <OverallCard group={data.overall} min={data.min_people} />
          <h2 className="pt-2 text-base font-bold">ケガごとのデータ</h2>
          <ul className="grid items-start gap-3 sm:grid-cols-2">
            {data.groups.map((group) => (
              <li key={group.body_part}>
                <GroupCard group={group} min={data.min_people} />
              </li>
            ))}
          </ul>
        </>
      )}
    </div>
  );
}
