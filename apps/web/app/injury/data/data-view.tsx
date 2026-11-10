"use client";
import { L } from "@/lib/i18n";

import { PageLoading } from "@/components/page-loading";

import Link from "next/link";
import { useEffect, useState } from "react";
import { EmptyState, ErrorState } from "@/components/empty-state";
import { Bars, hasItems } from "@/components/injury-public-summary";
import { apiFetch } from "@/lib/api";
import { ChevronDown } from "lucide-react";
import { injuryGroupPath, type InjuryPublicGroup, type InjuryTally } from "@/lib/injury";

type InjuryData = { min_people: number; overall: InjuryPublicGroup; groups: InjuryPublicGroup[] };

// 質問ごとの見出し。何を聞いた答えなのかが一目でわかるよう、質問の形にする。
function questionsOf(group: InjuryPublicGroup) {
  return [
    { title: L("ケガの前に、何か変わった？"), short: L("ケガの前の変化"), tally: group.pre_changes },
    { title: L("そのとき何をしていた？"), short: L("していたこと"), tally: group.activities },
    { title: L("どこでケガをした？"), short: L("場所"), tally: group.places },
    { title: L("種目は？"), short: L("種目"), tally: group.events },
    { title: L("いま思うと、どうすればよかった？"), short: L("どうすればよかったか"), tally: group.hindsight },
    { title: L("復帰までに何をした？"), short: L("復帰までにやったこと"), tally: group.actions },
  ];
}

function topItem(tally: InjuryTally) {
  return [...tally.items].sort((a, b) => b.people - a.people)[0];
}

function Details({ group }: { group: InjuryPublicGroup }) {
  return (
    <div className="space-y-4">
      {questionsOf(group).map((q) => (
        <Bars key={q.short} title={q.title} tally={q.tally} top={3} hidePeople />
      ))}
    </div>
  );
}

function hasData(group: InjuryPublicGroup) {
  return hasItems(...questionsOf(group).map((q) => q.tally));
}

// 全体のデータ。質問ごとの上位3つを出す。
function OverallCard({ group }: { group: InjuryPublicGroup }) {
  return (
    <section className="space-y-4 rounded-2xl border border-white/20 glass bg-white/10 p-4">
      <div className="flex items-baseline justify-between gap-3">
        <h2 className="text-lg font-bold">{L(group.body_part_label)}</h2>
      </div>
      {hasData(group) ? (
        <Details group={group} />
      ) : (
        <p className="text-sm text-slate-400">{L("データが集まると表示します")}</p>
      )}
    </section>
  );
}

// ケガごとのデータ。並べると長くなるので、閉じた状態で要点だけ見せ、開くと詳しく見られる。
function GroupCard({ group }: { group: InjuryPublicGroup }) {
  const ready = hasData(group);
  const first = topItem(group.pre_changes) ?? topItem(group.activities);
  return (
    <details className="group rounded-2xl border border-white/20 glass bg-white/10 p-4">
      <summary className="flex cursor-pointer list-none items-start justify-between gap-3">
        <span className="min-w-0">
          <span className="block font-bold">{L(group.body_part_label)}</span>
          <span className="mt-0.5 block text-xs text-slate-300">
            {ready ? (first ? L("多かったのは「{label}」", { label: L(first.label) }) : L("データを見る")) : L("データが集まると表示します")}
          </span>
        </span>
        <span className="flex shrink-0 items-center gap-1 text-sm text-slate-300">
          <ChevronDown className="size-4 transition group-open:rotate-180" aria-hidden />
        </span>
      </summary>
      <div className="mt-4 space-y-4">
        {ready ? <Details group={group} /> : null}
        <Link href={injuryGroupPath(group.body_part)} className="block text-sm text-blue-300 underline-offset-2 hover:underline">{L("このグループを見る →")}</Link>
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
        <p className="text-sm text-slate-200">{L("みんなの経験を自分のケガ対策に活かそう！")}</p>
      </div>

      {failed ? (
        <ErrorState message={L("データの取得に失敗しました。時間をおいて、もう一度開いてください。")} />
      ) : data === null ? (
        <PageLoading label={L("読み込み中...")} />
      ) : data.groups.length === 0 ? (
        <EmptyState title={L("まだデータがありません")} body={L("共有のグループに参加する人が増えると、ここに集計が出ます。")} />
      ) : (
        <>
          <OverallCard group={data.overall} />
          <h2 className="pt-2 text-base font-bold">{L("ケガごとのデータ")}</h2>
          <ul className="grid items-start gap-3 sm:grid-cols-2">
            {data.groups.map((group) => (
              <li key={group.body_part}>
                <GroupCard group={group} />
              </li>
            ))}
          </ul>
        </>
      )}
    </div>
  );
}
