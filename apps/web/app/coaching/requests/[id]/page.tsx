"use client";

import { useAuth } from "@/components/auth-provider";
import { apiFetch, ApiRequestError } from "@/lib/api";
import { coachEventLabels, coachTierLabels } from "@/lib/coach-recruit";
import { formatDateTime } from "@/lib/format";
import { reviewFocusLabels, reviewStatusLabels } from "@/lib/review";
import type { ReviewRequest } from "@/lib/types";
import Link from "next/link";
import { useParams, useRouter } from "next/navigation";
import { useEffect, useState } from "react";

// 送った添削の依頼。返答の表示はこの後の PR で足す。
export default function ReviewRequestPage() {
  const { session, token, loading } = useAuth();
  const router = useRouter();
  const { id } = useParams<{ id: string }>();
  const [request, setRequest] = useState<ReviewRequest | null>(null);
  const [error, setError] = useState("");

  useEffect(() => {
    if (!loading && !session) {
      router.replace(`/login?next=${encodeURIComponent(window.location.pathname)}`);
    }
  }, [loading, session, router]);

  useEffect(() => {
    if (!token || !id) return;
    let cancelled = false;
    apiFetch<ReviewRequest>(`/v1/coaching/requests/${id}`, { token })
      .then((d) => {
        if (!cancelled) setRequest(d);
      })
      .catch((err) => {
        if (!cancelled) setError(err instanceof ApiRequestError && err.status === 404 ? "依頼が見つかりません" : "読み込みに失敗しました");
      });
    return () => {
      cancelled = true;
    };
  }, [token, id]);

  if (error) return <p className="text-sm text-red-400">{error}</p>;
  if (loading || !session || !request) return <p className="text-sm text-slate-200">読み込み中...</p>;

  const waiting = request.status === "queued" || request.status === "offered" || request.status === "in_review";

  return (
    <div className="mx-auto max-w-md space-y-5">
      <div className="space-y-1">
        <h1 className="text-2xl font-bold">フォーム添削</h1>
        <p className="text-sm text-slate-300">送った日時 {formatDateTime(request.created_at)}</p>
      </div>

      <section className="space-y-1 rounded-xl border border-blue-400/40 bg-blue-500/15 p-3">
        <p className="text-sm font-semibold text-white">{reviewStatusLabels[request.status]}</p>
        {waiting ? <p className="text-sm text-slate-100">返ってくる目安: {formatDateTime(request.due_at)}まで</p> : null}
      </section>

      {request.video_url ? (
        <video
          src={request.video_url}
          poster={request.thumbnail_url || undefined}
          controls
          playsInline
          preload="metadata"
          className="w-full rounded-xl bg-black"
        />
      ) : null}

      <dl className="space-y-2 rounded-xl border border-white/20 glass bg-white/10 p-3 text-sm">
        <div>
          <dt className="text-xs text-slate-300">誰に見てもらうか</dt>
          <dd>{coachTierLabels[request.tier]}</dd>
        </div>
        <div>
          <dt className="text-xs text-slate-300">種目</dt>
          <dd>{coachEventLabels[request.event]}</dd>
        </div>
        <div>
          <dt className="text-xs text-slate-300">見てほしい点</dt>
          <dd>{request.focus.map((f) => reviewFocusLabels[f]).join("、")}</dd>
        </div>
      </dl>

      <Link href="/me/coaching/requests" className="block text-center text-sm text-blue-300 underline">
        依頼の一覧へ
      </Link>
    </div>
  );
}
