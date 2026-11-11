"use client";
import { L } from "@/lib/i18n";
import { useTitleTranslation } from "@/components/locale-provider";

import { PageLoading } from "@/components/page-loading";

import Link from "next/link";
import { useEffect, useState } from "react";
import { ErrorState } from "@/components/empty-state";
import { BackButton } from "@/components/major-hosts-panel";
import { apiFetch } from "@/lib/api";
import { type PrefecturePage } from "@/lib/prefecture-page";
import { prefectureName } from "@/lib/prefectures";
import { practiceListPath, practicePagePath, type PracticeList } from "@/lib/practice-list";

// この県の練習相手募集。件数だけを取りに行き、一覧は /practice で見る。件数が取れなくてもリンクは出す。
function PracticeSection({ code, name }: { code: string; name: string }) {
  const [total, setTotal] = useState<number | null>(null);
  useEffect(() => {
    const controller = new AbortController();
    apiFetch<PracticeList>(practiceListPath({ prefecture: code, event: "" }), { signal: controller.signal })
      .then((list) => setTotal(list.total))
      .catch(() => {});
    return () => controller.abort();
  }, [code]);
  return (
    <section className="space-y-2">
      <h2 className="text-lg font-semibold text-slate-100">{L("{name}の練習相手募集", { name })}</h2>
      {total !== null ? <p className="text-sm text-slate-200">{L("これからの募集 {total}件", { total })}</p> : null}
      <Link href={practicePagePath({ prefecture: code, event: "" })} className="inline-flex min-h-9 items-center text-sm text-blue-300 underline">{L("募集を見る")}</Link>
    </section>
  );
}

// 中身はサーバーで取ってから渡す(page.tsx)。サーバーで取れなかったときだけ、画面側で取り直す。
export function PrefectureView({ code, initialPage, titleJa = "" }: { code: string; initialPage: PrefecturePage | null; titleJa?: string }) {
  const [page, setPage] = useState(initialPage);
  const [failed, setFailed] = useState(false);
  const name = prefectureName(code);
  useTitleTranslation(titleJa, L("{name}のクラブチームの募集", { name }));

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
        <p className="mt-1 text-sm text-slate-200">{L("{name}のクラブチームの募集をまとめています。", { name })}</p>
      </div>

      {failed ? (
        <ErrorState message={L("取得に失敗しました。時間をおいて開き直してください。")} />
      ) : !page ? (
        <PageLoading label={L("読み込み中...")} />
      ) : (
        <PracticeSection code={code} name={name} />
      )}
      <BackButton />
    </div>
  );
}
