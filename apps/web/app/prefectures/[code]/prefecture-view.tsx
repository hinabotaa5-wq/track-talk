"use client";

import Link from "next/link";
import { useEffect, useState } from "react";
import { useAuth } from "@/components/auth-provider";
import { EmptyState, ErrorState } from "@/components/empty-state";
import { BackButton } from "@/components/major-hosts-panel";
import { apiFetch } from "@/lib/api";
import { formatRelativeTime } from "@/lib/format";
import { prefectureCompetitionsHref, prefectureRecruitmentsHref, type PrefecturePage } from "@/lib/prefecture-page";
import { prefectureName } from "@/lib/prefectures";
import { RequireSession } from "../../competitions/require-session";
import { competitionDates } from "../../competitions/regional/regional-view";

function MoreLink({ href, total, shown }: { href: string; total: number; shown: number }) {
  return (
    <Link href={href} className="inline-flex min-h-9 items-center text-sm text-blue-300 underline">
      {total > shown ? `もっと見る（全${total}件）` : "一覧で見る"}
    </Link>
  );
}

export function PrefectureView({ code }: { code: string }) {
  const { token } = useAuth();
  const [page, setPage] = useState<PrefecturePage | null>(null);
  const [failed, setFailed] = useState(false);
  const name = prefectureName(code);

  useEffect(() => {
    if (!token) return;
    const controller = new AbortController();
    apiFetch<PrefecturePage>(`/v1/prefectures/${code}`, { token, signal: controller.signal })
      .then(setPage)
      .catch(() => {
        if (!controller.signal.aborted) setFailed(true);
      });
    return () => controller.abort();
  }, [code, token]);

  return (
    <RequireSession>
      <div className="mx-auto max-w-2xl space-y-6">
        <div>
          <h1 className="text-2xl font-bold text-slate-100">{name}</h1>
          <p className="mt-1 text-sm text-slate-200">{name}のこれからの大会と、クラブチームの募集をまとめています。</p>
        </div>

        {failed ? (
          <ErrorState message="取得に失敗しました。APIの起動を確認してください。" />
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

            <section className="space-y-2">
              <h2 className="text-lg font-semibold text-slate-100">クラブチームの募集</h2>
              {page.recruitments.length === 0 ? (
                <EmptyState title="まだ募集はありません" body={`${name}で活動しているクラブチームは、募集を投稿できます。`} />
              ) : (
                <ul className="space-y-2">
                  {page.recruitments.map((r) => (
                    <li key={r.id}>
                      <Link
                        href={`/posts/${r.id}`}
                        className="block rounded-xl border border-white/20 glass bg-white/10 p-3.5 transition hover:bg-white/16 active:scale-[0.98]"
                      >
                        <p className="font-semibold text-slate-100">{r.title}</p>
                        <p className="text-sm text-slate-300">{[r.venue, r.schedule].filter(Boolean).join("・")}</p>
                        <p className="text-xs text-slate-400">{formatRelativeTime(r.created_at)}</p>
                      </Link>
                    </li>
                  ))}
                </ul>
              )}
              {page.recruitments.length > 0 ? (
                <MoreLink href={prefectureRecruitmentsHref(name)} total={page.recruitment_total} shown={page.recruitments.length} />
              ) : null}
            </section>
          </>
        )}
        <BackButton />
      </div>
    </RequireSession>
  );
}
