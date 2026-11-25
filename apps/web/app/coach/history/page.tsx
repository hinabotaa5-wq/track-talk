"use client";
import { L } from "@/lib/i18n";

import { useAuth } from "@/components/auth-provider";
import { CoachSubpageShell } from "@/components/coach-subpage";
import { PageLoading } from "@/components/page-loading";
import { apiFetch } from "@/lib/api";
import { coachEventLabels, coachTierLabels } from "@/lib/coach-recruit";
import { formatDateTime } from "@/lib/format";
import { reviewQuestionText, reviewVideosPurgedNote } from "@/lib/review";
import type { CoachRequests, CoachReviewRequest } from "@/lib/types";
import Link from "next/link";
import { useRouter } from "next/navigation";
import { useEffect, useState } from "react";

// 履歴の動画はミニプレーヤー（本体 #1014 補足3）。幅160px・高さ220pxを上限に、縦横比を保って黒地に収める。
const miniPlayerClass = "block max-h-[220px] max-w-[160px] rounded-lg bg-black object-contain";

// 返却済みの添削の履歴（本体 #1014）。返却済み・補足回答待ちを、返却の新しい順に並べ、詳細（自分の返答）へ戻れるようにする。
// 閲覧専用で、自動更新はしない。
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
    apiFetch<CoachRequests>("/v1/coach/requests", { token })
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
              <p className="font-semibold text-white">
                {coachEventLabels[request.event]} · {coachTierLabels[request.tier]}
              </p>
              <p className="whitespace-pre-wrap break-words text-sm text-slate-200">{reviewQuestionText(request)}</p>
              {request.returned_at ? (
                <p className="text-xs text-slate-300">{L("返却 {date}", { date: formatDateTime(request.returned_at) })}</p>
              ) : null}
            </div>
            <HistoryVideos request={request} />
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

// 選手の動画と見本動画を横に並べる（入りきらなければ折り返す）。動画を消した後は、削除済みの注記を1つだけ出す。
function HistoryVideos({ request }: { request: CoachReviewRequest }) {
  if (request.videos_purged) return <p className="text-xs text-slate-300">{reviewVideosPurgedNote()}</p>;
  if (!request.video_url && !request.demo_video_url) return null;
  return (
    <div className="flex flex-wrap items-start gap-2">
      {request.video_url ? (
        <video
          data-testid="history-video"
          src={request.video_url}
          poster={request.thumbnail_url || undefined}
          controls
          playsInline
          preload="metadata"
          className={miniPlayerClass}
        />
      ) : null}
      {request.demo_video_url ? (
        <figure className="space-y-1">
          <figcaption className="text-xs text-slate-300">{L("見本動画")}</figcaption>
          <video
            data-testid="history-demo-video"
            src={request.demo_video_url}
            controls
            playsInline
            preload="metadata"
            className={miniPlayerClass}
          />
        </figure>
      ) : null}
    </div>
  );
}
