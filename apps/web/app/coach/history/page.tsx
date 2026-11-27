"use client";
import { PremiumBadge } from "@/components/premium-review";
import { isPremium } from "@/lib/premium";
import { L } from "@/lib/i18n";

import { useAuth } from "@/components/auth-provider";
import { CoachSubpageShell } from "@/components/coach-subpage";
import { PageLoading } from "@/components/page-loading";
import { apiFetch } from "@/lib/api";
import { coachEventLabels, coachTierLabels } from "@/lib/coach-recruit";
import { formatDateTime } from "@/lib/format";
import { reviewQuestionText } from "@/lib/review";
import type { CoachRequests, CoachReviewRequest } from "@/lib/types";
import Link from "next/link";
import { useRouter } from "next/navigation";
import { useEffect, useState } from "react";

// 返却済みの添削の履歴（本体 #1014）。返却済み・補足回答待ちを、返却の新しい順に並べ、詳細（自分の返答）へ戻れるようにする。
// 閲覧専用で、自動更新はしない。一覧には動画を出さず、動画と自分の返答は詳細を開いたときに見る（運営者決定 2026-11-25）。
export default function CoachHistoryPage() {
  const { session, token, loading } = useAuth();
  const router = useRouter();
  const [rows, setRows] = useState<CoachReviewRequest[] | null>(null);
  const [error, setError] = useState("");

  useEffect(() => {
    if (!loading && !session) router.replace(`/login?next=${encodeURIComponent("/coach/history")}`);
  }, [loading, session, router]);

  useEffect(() => {
    if (!token) return;
    let cancelled = false;
    apiFetch<CoachRequests>("/v1/coach/requests?scope=completed", { token })
      .then((d) => { if (!cancelled) setRows(d.completed ?? []); })
      .catch(() => { if (!cancelled) setError(L("履歴を読み込めませんでした")); });
    return () => { cancelled = true; };
  }, [token]);

  if (loading || !session || (rows === null && !error)) return <PageLoading screen label={L("読み込み中...")} />;

  return (
    <CoachSubpageShell title={L("履歴")}>
      {error ? <p role="alert" className="text-sm text-red-400">{error}</p> : null}
      {rows?.length === 0 ? <p className="text-sm text-slate-200">{L("まだ返却した添削はありません。")}</p> : null}
      <ul className="space-y-3">
        {rows?.map((request) => (
          <li key={request.id} className="space-y-3 rounded-xl border border-white/20 glass bg-white/10 p-3">
            <div className="space-y-0.5">
              <p className="flex flex-wrap items-center gap-2 font-semibold text-white">
                <span>{coachEventLabels[request.event]} · {coachTierLabels[request.tier]}</span>
                {isPremium(request) ? <PremiumBadge /> : null}
              </p>
              {isPremium(request) ? null : <p className="whitespace-pre-wrap break-words text-sm text-slate-200">{reviewQuestionText(request)}</p>}
              {request.returned_at ? (
                <p className="text-xs text-slate-300">{L("返却 {date}", { date: formatDateTime(request.returned_at) })}</p>
              ) : null}
            </div>
            <Link
              href={`/coach/requests/${request.id}`}
              className="flex min-h-11 w-full items-center justify-center rounded-md bg-blue-600 px-4 text-sm font-semibold text-pure-white"
            >
              {L("開く")}
            </Link>
          </li>
        ))}
      </ul>
    </CoachSubpageShell>
  );
}
