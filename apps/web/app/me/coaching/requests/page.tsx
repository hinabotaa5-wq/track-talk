"use client";

import { useAuth } from "@/components/auth-provider";
import { apiFetch } from "@/lib/api";
import { coachEventLabels } from "@/lib/coach-recruit";
import { formatDateTime } from "@/lib/format";
import { reviewStatusLabels } from "@/lib/review";
import type { Meta, ReviewRequest } from "@/lib/types";
import Link from "next/link";
import { useRouter } from "next/navigation";
import { useEffect, useState } from "react";

// 自分が送ったフォーム添削の依頼の一覧。
export default function MyReviewRequestsPage() {
  const { session, token, loading } = useAuth();
  const router = useRouter();
  const [requests, setRequests] = useState<ReviewRequest[] | null>(null);
  const [error, setError] = useState("");

  useEffect(() => {
    if (!loading && !session) {
      router.replace(`/login?next=${encodeURIComponent(window.location.pathname)}`);
    }
  }, [loading, session, router]);

  useEffect(() => {
    if (!token) return;
    let cancelled = false;
    apiFetch<{ requests: ReviewRequest[]; meta: Meta }>("/v1/me/coaching/requests?per_page=50", { token })
      .then((d) => {
        if (!cancelled) setRequests(d.requests ?? []);
      })
      .catch(() => {
        if (!cancelled) setError("読み込みに失敗しました");
      });
    return () => {
      cancelled = true;
    };
  }, [token]);

  if (error) return <p className="text-sm text-red-400">{error}</p>;
  if (loading || !session || !requests) return <p className="text-sm text-slate-200">読み込み中...</p>;

  return (
    <div className="mx-auto max-w-md space-y-4">
      <h1 className="text-2xl font-bold">フォーム添削の依頼</h1>
      {requests.length === 0 ? <p className="text-sm text-slate-200">まだ依頼はありません</p> : null}
      <ul className="space-y-2">
        {requests.map((r) => (
          <li key={r.id}>
            <Link
              href={`/coaching/requests/${r.id}`}
              className="flex items-center justify-between gap-3 rounded-xl border border-white/20 glass bg-white/10 p-3"
            >
              <span className="space-y-0.5">
                <span className="block text-sm font-semibold text-white">{coachEventLabels[r.event]}</span>
                <span className="block text-xs text-slate-300">{formatDateTime(r.created_at)}</span>
              </span>
              <span className="shrink-0 text-xs text-blue-300">{reviewStatusLabels[r.status]}</span>
            </Link>
          </li>
        ))}
      </ul>
      <Link
        href="/coaching/new"
        className="flex min-h-11 w-full items-center justify-center rounded-md bg-blue-600 px-4 text-sm font-semibold text-white"
      >
        新しく頼む
      </Link>
    </div>
  );
}
