"use client";

import { useEffect, useState } from "react";
import { apiFetch } from "@/lib/api";
import { formatDate } from "@/lib/format";
import { injuryProgressMedianText, injuryProgressMyText, type InjuryProgress } from "@/lib/injury";

// 自分のケガの経過を、同じケガのみんなの中央値と段階ごとに並べる。
export function InjuryProgressCompare({ token }: { token: string }) {
  const [progress, setProgress] = useState<InjuryProgress | null>(null);

  useEffect(() => {
    const controller = new AbortController();
    apiFetch<InjuryProgress>("/v1/injury/me/progress", { token, signal: controller.signal })
      .then(setProgress)
      .catch(() => {
        // 比較は補足の情報なので、取得できなければ何も出さない。
      });
    return () => controller.abort();
  }, [token]);

  return <InjuryProgressCompareView progress={progress} />;
}

export function InjuryProgressCompareView({ progress }: { progress: InjuryProgress | null }) {
  if (!progress || progress.posts.length === 0) return null;

  return (
    <section className="space-y-3">
      <h2 className="text-lg font-bold">みんなの目安との比較</h2>
      <ul className="space-y-3">
        {progress.posts.map((post) => (
          <li key={post.post_id} className="space-y-2 rounded-xl border border-white/20 glass bg-white/10 p-4">
            <p className="flex flex-wrap items-center gap-2 text-xs">
              <span className="rounded-full border border-white/20 bg-white/12 px-2.5 py-0.5 font-semibold text-slate-200">
                {post.body_part_label}
              </span>
              <span className="text-slate-300">{formatDate(post.posted_on)}に投稿</span>
            </p>
            <table className="w-full text-sm">
              <thead>
                <tr className="text-xs text-slate-300">
                  <th className="pb-1 text-left font-medium">段階</th>
                  <th className="pb-1 text-right font-medium">自分</th>
                  <th className="pb-1 text-right font-medium">みんなの中央値</th>
                </tr>
              </thead>
              <tbody>
                {post.stages.map((stage) => (
                  <tr key={stage.stage}>
                    <th scope="row" className="whitespace-nowrap py-0.5 text-left font-medium text-slate-100">
                      {stage.stage_label}
                    </th>
                    <td className={`whitespace-nowrap py-0.5 pl-2 text-right ${stage.my_days === null ? "text-slate-400" : "text-emerald-300"}`}>
                      {injuryProgressMyText(stage)}
                    </td>
                    <td className={`whitespace-nowrap py-0.5 pl-2 text-right ${stage.median_days === null ? "text-xs text-slate-400" : "text-slate-200"}`}>
                      {injuryProgressMedianText(stage, progress.min_sample)}
                    </td>
                  </tr>
                ))}
              </tbody>
            </table>
          </li>
        ))}
      </ul>
      <p className="text-xs text-slate-300">
        日数は投稿の日からその段階を最初に報告した日まで。みんなの経過報告の中央値で、回復を保証するものではありません。
      </p>
    </section>
  );
}
