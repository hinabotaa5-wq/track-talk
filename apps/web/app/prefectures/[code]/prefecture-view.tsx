"use client";

import Link from "next/link";
import { useEffect, useState } from "react";
import { EmptyState, ErrorState } from "@/components/empty-state";
import { BackButton } from "@/components/major-hosts-panel";
import { apiFetch } from "@/lib/api";
import { prefectureCompetitionsHref, type PrefecturePage } from "@/lib/prefecture-page";
import { prefectureName } from "@/lib/prefectures";
import { competitionDates } from "../../competitions/regional/regional-view";

function MoreLink({ href, total, shown }: { href: string; total: number; shown: number }) {
  return (
    <Link href={href} className="inline-flex min-h-9 items-center text-sm text-blue-300 underline">
      {total > shown ? `もっと見る（全${total}件）` : "一覧で見る"}
    </Link>
  );
}

// 中身はサーバーで取ってから渡す(page.tsx)。サーバーで取れなかったときだけ、画面側で取り直す。
export function PrefectureView({ code, initialPage }: { code: string; initialPage: PrefecturePage | null }) {
  const [page, setPage] = useState(initialPage);
  const [failed, setFailed] = useState(false);
  const name = prefectureName(code);

  useEffect(() => {
    if (initialPage) return;
    const controller = new AbortController();
    apiFetch<PrefecturePage>(`/v1/prefectures/${code}`, { signal: controller.signal })
      .then(setPage)
      .catch(() => {
        if (!controller.signal.aborted) setFailed(true);
      });
    return () => controller.abort();
  }, [code, initialPage]);

  return (
    <div className="mx-auto max-w-2xl space-y-6">
      <div>
        <h1 className="text-2xl font-bold text-slate-100">{name}</h1>
        <p className="mt-1 text-sm text-slate-200">{name}のこれからの大会をまとめています。</p>
      </div>

      {failed ? (
        <ErrorState message="取得に失敗しました。時間をおいて開き直してください。" />
      ) : !page ? (
        <p className="text-sm text-slate-200">読み込み中...</p>
      ) : (
        <>
          <section className="space-y-2">
            <h2 className="text-lg font-semibold text-slate-100">これからの大会</h2>
            {page.competitions.length === 0 ? (
              <EmptyState title="これからの大会はまだありません" body="大会の日程は、都道府県の陸協のページから順次取り込みます。" />
            ) : (
              <ul className="space-y-2">
                {page.competitions.map((c) => (
                  <li key={c.id} className="rounded-xl border border-white/20 glass bg-white/10 p-3.5">
                    <p className="text-xs text-slate-200">{competitionDates(c.starts_on, c.ends_on)}</p>
                    <p className="font-semibold text-slate-100">{c.name}</p>
                    {c.location ? <p className="text-sm text-slate-300">{c.location}</p> : null}
                  </li>
                ))}
              </ul>
            )}
            <MoreLink href={prefectureCompetitionsHref(code, page.competitions[0]?.starts_on)} total={page.competition_total} shown={page.competitions.length} />
          </section>
        </>
      )}
      <BackButton />
    </div>
  );
}
