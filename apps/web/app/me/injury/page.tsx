"use client";

import Link from "next/link";
import { useRouter } from "next/navigation";
import { useEffect, useState } from "react";
import { useAuth } from "@/components/auth-provider";
import { EmptyState, ErrorState } from "@/components/empty-state";
import { apiFetch } from "@/lib/api";
import { formatDateTime } from "@/lib/format";
import { injuryHistoryStageText, injuryRepeatedParts, type InjuryHistoryItem } from "@/lib/injury";

export default function MyInjuryHistoryPage() {
  const { session, token, loading } = useAuth();
  const router = useRouter();
  const [items, setItems] = useState<InjuryHistoryItem[] | null>(null);
  const [failed, setFailed] = useState(false);

  useEffect(() => {
    if (!loading && !session) router.replace("/login?next=/me/injury");
  }, [loading, session, router]);

  useEffect(() => {
    if (!token) return;
    let active = true;
    apiFetch<{ items: InjuryHistoryItem[] }>("/v1/me/injury-history", { token })
      .then((d) => {
        if (active) setItems(d.items ?? []);
      })
      .catch(() => {
        if (active) setFailed(true);
      });
    return () => {
      active = false;
    };
  }, [token]);

  if (loading || !session) return <p className="text-sm text-slate-200">読み込み中...</p>;

  const repeated = items ? injuryRepeatedParts(items) : [];

  return (
    <div className="space-y-4">
      <div>
        <Link href="/me" className="text-sm text-slate-200 underline-offset-2 hover:underline">
          ← マイページ
        </Link>
        <h1 className="mt-2 text-2xl font-bold">自分のケガ履歴</h1>
        <p className="mt-1 text-sm text-slate-200">
          これまでのケガの投稿と、経過を報告した段階ごとの日数を振り返れます。あなただけに表示されます。
        </p>
      </div>

      {failed ? (
        <ErrorState message="履歴の取得に失敗しました。APIの起動を確認してください。" />
      ) : items === null ? (
        <p className="text-sm text-slate-200">読み込み中...</p>
      ) : items.length === 0 ? (
        <EmptyState title="まだケガの投稿がありません" body="ケガの溜まり場に投稿すると、ここに履歴がたまります。" />
      ) : (
        <>
          {repeated.length > 0 ? (
            <p className="rounded-xl border border-amber-700/60 bg-amber-950/40 p-3 text-sm text-amber-200">
              {repeated.map((item) => `${item.label}（${item.count}回）`).join("、")}は、これまでに複数回ケガの投稿があります。
            </p>
          ) : null}
          <ul className="space-y-3">
            {items.map((item) => (
              <li key={item.id} className="rounded-xl border border-white/20 glass bg-white/10 p-4">
                <Link href={`/posts/${item.id}`} className="block space-y-2">
                  <p className="flex flex-wrap items-center gap-2 text-xs">
                    <span className="rounded-full border border-white/20 bg-white/12 px-2.5 py-0.5 font-semibold text-slate-200">
                      {item.body_part_label}
                    </span>
                    <span className="text-slate-300">{formatDateTime(item.created_at)}</span>
                  </p>
                  <h2 className="font-semibold text-slate-100">{item.title}</h2>
                  {item.stages.length > 0 ? (
                    <ol className="flex flex-wrap gap-x-3 gap-y-1 text-sm text-emerald-300">
                      {item.stages.map((stage) => (
                        <li key={stage.stage}>{injuryHistoryStageText(stage)}</li>
                      ))}
                    </ol>
                  ) : (
                    <p className="text-sm text-slate-300">経過の報告はまだありません。</p>
                  )}
                </Link>
              </li>
            ))}
          </ul>
        </>
      )}
    </div>
  );
}
