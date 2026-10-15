"use client";

import Link from "next/link";
import { useEffect, useState } from "react";
import { EmptyState, ErrorState } from "@/components/empty-state";
import { apiFetch } from "@/lib/api";
import { injuryStageStatText, type InjuryStats } from "@/lib/injury";

export function InjuryStatsView() {
  const [stats, setStats] = useState<InjuryStats | null>(null);
  const [failed, setFailed] = useState(false);

  useEffect(() => {
    const controller = new AbortController();
    apiFetch<InjuryStats>("/v1/injury/stats", { signal: controller.signal })
      .then(setStats)
      .catch(() => {
        if (!controller.signal.aborted) setFailed(true);
      });
    return () => controller.abort();
  }, []);

  return (
    <div className="space-y-4">
      <div>
        <Link href="/injury" className="text-sm text-slate-200 underline-offset-2 hover:underline">
          ← ケガ
        </Link>
        <h1 className="mt-2 text-2xl font-bold">みんなの復帰データ</h1>
        <p className="mt-1 text-sm text-slate-200">
          投稿された経過報告を、ケガごとに匿名で集計しています。日数は「ケガの投稿から、その段階を最初に報告するまで」の中央値で、
          報告が{stats?.min_sample ?? 5}人そろった段階だけ表示します。
        </p>
      </div>

      {failed ? (
        <ErrorState message="データの取得に失敗しました。APIの起動を確認してください。" />
      ) : stats === null ? (
        <p className="text-sm text-slate-200">読み込み中...</p>
      ) : stats.parts.length === 0 ? (
        <EmptyState title="まだデータがありません" body="ケガの投稿と経過報告が増えると、ここに集計が出ます。" />
      ) : (
        <ul className="grid gap-3 sm:grid-cols-2">
          {stats.parts.map((part) => (
            <li key={part.body_part} className="rounded-xl border border-white/20 glass bg-white/10 p-4">
              <div className="flex items-baseline justify-between gap-2">
                <h2 className="text-lg font-semibold">{part.body_part_label}</h2>
                <span className="text-sm text-slate-200">{part.posts}件の投稿</span>
              </div>
              {part.stages.length === 0 ? (
                <p className="mt-2 text-sm text-slate-300">経過の報告はまだありません。</p>
              ) : (
                <dl className="mt-2 space-y-1.5 text-sm">
                  {part.stages.map((stage) => (
                    <div key={stage.stage} className="flex justify-between gap-3">
                      <dt className="font-medium text-slate-100">{stage.stage_label}</dt>
                      <dd className="text-right text-slate-200">{injuryStageStatText(stage, stats.min_sample)}</dd>
                    </div>
                  ))}
                </dl>
              )}
            </li>
          ))}
        </ul>
      )}

    </div>
  );
}
